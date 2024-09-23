package main

import (
	"bytes"
	"embed"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
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
	"github.com/kljensen/snowball"
)

func stemFromString(str string) string {
	result := strings.TrimSpace(str)
	if stemmed, err := snowball.Stem(result, "english", true); err == nil {
		result = stemmed
	}
	return result
}

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
					str = stemFromString(str)
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

type stringScoreMap map[string]float64
type stringScorePair struct {
	string string
	score  float64
}

type document struct {
	Filepath         string
	Terms            []string
	TfidfVector      stringScoreMap
	CosineSimilarity float64
}

func (doc document) MarshalJSON() ([]byte, error) {
	type documentJson struct {
		Filepath string  `json:"filepath"`
		Score    float64 `json:"score"`
	}

	docJson := documentJson{doc.Filepath, doc.CosineSimilarity}

	var byteBuffer bytes.Buffer
	encoder := json.NewEncoder(&byteBuffer)
	err := encoder.Encode(docJson)
	return byteBuffer.Bytes(), err
}

func (tfidf stringScoreMap) vectorFromQuery(query queryParseResult) stringScoreMap {
	result := make(stringScoreMap)

	for _, needle := range query.fzyNeedles {
		mapGetResult := fzy.MapGet(tfidf, needle)
		bestPossibleFzyScore := uint64(float64(fzy.BestScoreFromNeedle(needle)) * 0.8)
		if mapGetResult.ScoreResult.Score >= bestPossibleFzyScore {
			result[mapGetResult.Key] = mapGetResult.Value
		}
	}

	for _, needle := range query.needles {
		if score, ok := tfidf[needle]; ok {
			result[needle] = score
		}
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
					Filepath:    filepath,
					Terms:       termsFromString(content),
					TfidfVector: make(stringScoreMap),
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
						doc.TfidfVector[term] += 1.0
					}

					seen := make(map[string]bool)
					termCount := float64(len(doc.Terms))
					for term := range doc.TfidfVector {
						if !seen[term] {
							seen[term] = true
							mutex.Lock()
							numberOfDocumentsContainingTermT[term] += 1.0
							mutex.Unlock()
						}

						doc.TfidfVector[term] /= termCount
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
					for key := range doc.TfidfVector {
						idf := math.Log(documentCount / numberOfDocumentsContainingTermT[key])
						doc.TfidfVector[key] *= idf
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

type queryParseResult struct {
	needles    []string
	fzyNeedles []string
}

func queryParse(query string) queryParseResult {
	var result queryParseResult

	query = strings.TrimSpace(query)

	type LexState struct {
		insideFzy bool
		isFzy     bool
	}

	var lexState LexState
	for i := 0; i < len(query); i += 1 {
		lexState.isFzy = false

		var begin, end uint

		for i < len(query) && (!unicode.IsLetter(rune(query[i])) && query[i] != '*') {
			i += 1
		}

		if lexState.insideFzy || query[i] == '*' {
			if !lexState.insideFzy {
				i += 1
			}
			begin = uint(i)
			lexState.isFzy = true
			lexState.insideFzy = true
			for ; i <= len(query); i += 1 {
				if i >= len(query) || (!unicode.IsLetter(rune(query[i])) && query[i] != '*') {
					end = uint(i)
					break
				}
				if query[i] == '*' {
					lexState.insideFzy = false
					end = uint(i)
					i += 1
					break
				}
			}

			goto end_tokenizing
		}

		if unicode.IsLetter(rune(query[i])) {
			begin = uint(i)
			for ; i <= len(query); i += 1 {
				if i >= len(query) {
					end = uint(len(query))
					break
				}
				if !unicode.IsLetter(rune(query[i])) {
					end = uint(i)
					break
				}
			}
			goto end_tokenizing
		}

	end_tokenizing:
		{
			str := query[begin:end]
			if lexState.isFzy {
				result.fzyNeedles = append(result.fzyNeedles, str)
			} else {
				stem := stemFromString(str)
				result.needles = append(result.needles, stem)
			}
		}
	}

	// fmt.Println("Exact Needles")
	// for _, needle := range result.needles {
	// 	fmt.Printf("  \"%v\"\n", needle)
	// }
	// fmt.Println("Fzy Needles")
	// for _, needle := range result.fzyNeedles {
	// 	fmt.Printf("  \"%v\"\n", needle)
	// }
	// fmt.Println()

	return result
}

const PORT = "8080"

//go:embed web_interface/*
var webInterface embed.FS

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
		webInterfaceWithoutParentFolder, err := fs.Sub(webInterface, "web_interface")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not create sub fs from embedded web interface: %v\n", err)
			os.Exit(1)
		}
		http.Handle("/", http.FileServerFS(webInterfaceWithoutParentFolder))

		http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				query := r.URL.Query().Get("query")

				if len(query) == 0 {
					w.WriteHeader(200)
					return
				}

				queryParseResult := queryParse(query)

				// Sort Documents by needle
				{
					for i := range documents {
						doc := &documents[i]
						doc.CosineSimilarity = 0.0

						queryVector := doc.TfidfVector.vectorFromQuery(queryParseResult)

						// calculate cosine similarity between query and document
						{
							var dotProduct, normA, normB float64

							for term, a := range doc.TfidfVector {
								if b, ok := queryVector[term]; ok {
									dotProduct += a * b
								}
								normA += a * a
							}

							for _, b := range queryVector {
								normB += b * b
							}

							if normA != 0.0 && normB != 0.0 {
								doc.CosineSimilarity = dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
							}
						}
					}

					sort.Slice(documents, func(i, j int) bool {
						return documents[j].CosineSimilarity < documents[i].CosineSimilarity
					})
				}

				var byteBuffer bytes.Buffer
				encoder := json.NewEncoder(&byteBuffer)
				encoder.Encode(documents[:min(len(documents), 100)])
				w.Write(byteBuffer.Bytes())
			}
		})

		fmt.Printf("listening on port %v\n", PORT)
		log.Fatal(http.ListenAndServe(":"+PORT, nil))
	}
}
