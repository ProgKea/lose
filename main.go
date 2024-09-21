package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/ProgKea/lose/fzy"
	"github.com/ProgKea/lose/txt"
)

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

type StringScoreMap map[string]float64
type StringScorePair struct {
	String string
	Score  float64
}

type StringScorePairs []StringScorePair

type Document struct {
	filepath string
	terms    []string
	tfidf    StringScoreMap
}

func (doc Document) scoreFromNeedle(needle string) float64 {
	var result float64

	if score, ok := doc.tfidf[needle]; ok {
		result = score
	} else {
		type Match struct {
			Score float64
			fzy.ScoreResult
		}
		bestPossibleFzyScore := fzy.BestScoreFromNeedle(needle)
		leeway := uint64(float64(len(needle)) * 0.3)
		scoreWithLeeway := bestPossibleFzyScore - leeway

		bestMatch := Match{}
		for term, score := range doc.tfidf {
			fzyMatch := fzy.Score(term, needle)
			if fzyMatch.Score >= scoreWithLeeway && fzy.ScoreResultLess(bestMatch.ScoreResult, fzyMatch) {
				bestMatch = Match{score, fzyMatch}
			}
		}
		result = bestMatch.Score
	}

	return result
}

var (
	needles []string
)

func main() {
	// Parse CLI Args
	{
		var needle string
		flag.StringVar(&needle, "needle", "", "")
		flag.Parse()

		if len(needle) == 0 {
			Logger.Error.Fatalln("No needle was provided")
		}
		needles = strings.Fields(needle)
	}

	// Gather documents
	var documents []Document
	{
		var filepaths []string
		root := "./docs/docs.gl"
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
			Logger.Error.Printf("Error walking path: \"%v\": %v\n", root, err)
		}

		for _, filepath := range filepaths {
			documents = append(documents, Document{
				filepath: filepath,
				terms:    termsFromString(txt.FromFilepath(filepath)),
				tfidf:    make(StringScoreMap),
			})
		}
	}

	// TFIDF
	{
		var wg sync.WaitGroup

		// Calculate TF, number of documents containg term t
		numberOfDocumentsContainingTermT := make(StringScoreMap)
		var mutex sync.Mutex

		// TF Pass
		{
			for i := range documents {
				wg.Add(1)

				doc := &documents[i]
				go func(doc *Document) {
					defer wg.Done()
					for _, term := range doc.terms {
						doc.tfidf[term] += 1.0
					}

					seen := make(map[string]bool)
					termCount := float64(len(doc.terms))
					for term := range doc.tfidf {
						if !seen[term] {
							seen[term] = true
							mutex.Lock()
							numberOfDocumentsContainingTermT[term] += 1.0
							mutex.Unlock()
						}

						doc.tfidf[term] /= termCount
					}
				}(doc)
			}

			wg.Wait()
		}

		// IDF Pass
		{
			documentCount := float64(len(documents))
			for i := range documents {
				wg.Add(1)

				doc := &documents[i]
				go func(doc *Document) {
					defer wg.Done()
					for key := range doc.tfidf {
						idf := math.Log(documentCount / numberOfDocumentsContainingTermT[key])
						doc.tfidf[key] *= idf
					}
				}(doc)
			}

			wg.Wait()
		}
	}

	// Sort Documents by needle
	{
		sort.Slice(documents, func(i, j int) bool {
			documentA := documents[i]
			documentB := documents[j]

			scoreA := 0.0
			scoreB := 0.0

			for _, needle := range needles {
				scoreA += documentA.scoreFromNeedle(needle)
				scoreB += documentB.scoreFromNeedle(needle)
			}

			return scoreB < scoreA
		})
	}

	for _, document := range documents[:min(10, len(documents))] {
		fmt.Println(document.filepath)
	}
}
