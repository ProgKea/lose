package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/ProgKea/lose/fzy"
	"github.com/ProgKea/lose/txt"
)

const IndexFilepath = "lose.index"

func termsFromString(str string) []string {
	var result []string

	// Fill result
	{
		var sb strings.Builder
		for _, codepoint := range str {
			if !unicode.IsLetter(codepoint) {
				if sb.Len() > 0 {
					str := strings.ToLower(sb.String())
					result = append(result, str)
					sb.Reset()
				}
			} else {
				sb.WriteRune(codepoint)
			}
		}
	}

	return result
}

type logger struct {
	Info  *log.Logger
	Warn  *log.Logger
	Error *log.Logger
}

func NewLogger() *logger {
	return &logger{
		Info:  log.New(os.Stdout, "info: ", 0),
		Warn:  log.New(os.Stdout, "warn: ", 0),
		Error: log.New(os.Stderr, "error: ", 0),
	}
}

var (
	Logger *logger
)

func init() {
	Logger = NewLogger()
}

type stringScoreMap map[string]float64
type stringScorePair struct {
	string string
	score  float64
}

type document struct {
	Filepath string
	Terms    []string
	Tfidf    stringScoreMap
	Score    float64
}

func (doc document) MarshalJSON() ([]byte, error) {
	type documentJson struct {
		Filepath string  `json:"filepath"`
		Score    float64 `json:"score"`
	}

	docJson := documentJson{doc.Filepath, doc.Score}

	var byteBuffer bytes.Buffer
	encoder := json.NewEncoder(&byteBuffer)
	err := encoder.Encode(docJson)
	return byteBuffer.Bytes(), err
}

func (doc document) scoreFromNeedle(needle string) float64 {
	var result float64

	if score, ok := doc.Tfidf[needle]; ok {
		result = score
	} else {
		type match struct {
			score float64
			fzy.ScoreResult
		}
		bestPossibleFzyScore := fzy.BestScoreFromNeedle(needle)
		scoreWithLeeway := uint64(float64(bestPossibleFzyScore) * 0.8)

		bestMatch := match{}
		for term, score := range doc.Tfidf {
			fzyMatch := fzy.Score(term, needle)
			if fzyMatch.Score >= scoreWithLeeway && fzy.ScoreResultLess(bestMatch.ScoreResult, fzyMatch) {
				bestMatch = match{score, fzyMatch}
			}
		}
		result = bestMatch.score
	}

	return result
}

var (
	needles   []string
	indexRoot string
	serve     bool
)

func index(root string) ([]document, error) {
	// Gather documents
	var documents []document
	{
		var filepaths []string
		root := root
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				filepaths = append(filepaths, path)
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("Error walking path: \"%v\": %v\n", root, err)
		}

		for _, filepath := range filepaths {
			content, err := txt.FromFilepath(filepath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: skipping \"%v\": %v\n", filepath, err)
			} else {
				documents = append(documents, document{
					Filepath: filepath,
					Terms:    termsFromString(content),
					Tfidf:    make(stringScoreMap),
				})
			}
		}
	}

	// TFIDF
	{
		var wg sync.WaitGroup

		// Calculate TF, number of documents containg term t
		numberOfDocumentsContainingTermT := make(stringScoreMap)
		var mutex sync.Mutex

		// TF Pass
		{
			for i := range documents {
				wg.Add(1)

				go func(doc *document) {
					defer wg.Done()
					for _, term := range doc.Terms {
						doc.Tfidf[term] += 1.0
					}

					seen := make(map[string]bool)
					termCount := float64(len(doc.Terms))
					for term := range doc.Tfidf {
						if !seen[term] {
							seen[term] = true
							mutex.Lock()
							numberOfDocumentsContainingTermT[term] += 1.0
							mutex.Unlock()
						}

						doc.Tfidf[term] /= termCount
					}
				}(&documents[i])
			}

			wg.Wait()
		}

		// IDF Pass
		{
			documentCount := float64(len(documents))
			for i := range documents {
				wg.Add(1)

				go func(doc *document) {
					defer wg.Done()
					for key := range doc.Tfidf {
						idf := math.Log(documentCount / numberOfDocumentsContainingTermT[key])
						doc.Tfidf[key] *= idf
					}
				}(&documents[i])
			}

			wg.Wait()
		}
	}

	// cache indexed documents
	{
		file, err := os.Create(IndexFilepath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not open index file \"%v\": %v\n", IndexFilepath, err)
			os.Exit(1)
		}
		encoder := gob.NewEncoder(file)
		if err := encoder.Encode(documents); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to serialize documents: %v\n", err)
		}
	}

	return documents, nil
}

func docsFromIndexFile() ([]document, error) {
	var result []document

	indexData, err := os.ReadFile(IndexFilepath)
	if err != nil {
		return nil, err
	}
	indexReader := bytes.NewReader(indexData)
	decoder := gob.NewDecoder(indexReader)

	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

const PORT = "8080"

func main() {
	// Parse CLI Args
	{
		var needle string
		flag.StringVar(&needle, "needle", "", "Your search query.")
		flag.StringVar(&indexRoot, "index", "", "Path you want to index.")
		flag.BoolVar(&serve, "serve", false, "serve a web interface.")
		flag.Parse()
		needles = strings.Fields(needle)
	}

	if _, err := os.Stat(IndexFilepath); len(indexRoot) == 0 && errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "error: no index file was found. Use the -index flag to create one.")
		flag.Usage()
		os.Exit(1)
	}

	var documents []document
	if len(indexRoot) == 0 {
		var err error
		documents, err = docsFromIndexFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not read index file \"%v\": %v\n", IndexFilepath, err)
			os.Exit(1)
		}
	} else {
		var err error
		documents, err = index(indexRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to index: %v\n", err)
			os.Exit(1)
		}
	}

	if serve {
		http.Handle("/", http.FileServer(http.Dir("web_interface")))

		http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				needle := r.URL.Query().Get("needle")

				if len(needle) == 0 {
					w.WriteHeader(200)
					return
				}

				needles = strings.Fields(needle)

				// Sort Documents by needle
				{
					for i := range documents {
						doc := &documents[i]
						score := 0.0
						for _, needle := range needles {
							score += doc.scoreFromNeedle(needle)
						}
						doc.Score = score
					}

					sort.Slice(documents, func(i, j int) bool {
						return documents[j].Score < documents[i].Score
					})
				}

				var byteBuffer bytes.Buffer
				encoder := json.NewEncoder(&byteBuffer)
				encoder.Encode(documents[:min(len(documents), 10)])
				w.Write(byteBuffer.Bytes())
			}
		})

		fmt.Printf("Listening on port: %v\n", PORT)
		log.Fatal(http.ListenAndServe(":"+PORT, nil))
	}
}

// TODO: Search for synonyms aswell
