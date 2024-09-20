package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/ProgKea/lose/fzy"
	"golang.org/x/net/html"
)

func extractHtmlText(htmlData string) string {
	var stringBuilder strings.Builder

	tokenizer := html.NewTokenizer(strings.NewReader(string(htmlData)))
	prevStartToken := tokenizer.Token()
loop:
	for {
		token := tokenizer.Next()
		switch {
		case token == html.ErrorToken:
			break loop
		case token == html.StartTagToken:
			prevStartToken = tokenizer.Token()
		case token == html.TextToken:
			if prevStartToken.Data == "script" {
				continue
			}
			txt := strings.TrimSpace(html.UnescapeString(string(tokenizer.Text())))
			if len(txt) > 0 {
				stringBuilder.WriteString(txt)
			}
		}
	}

	return stringBuilder.String()
}

func stringToTerms(str string) []string {
	var result []string
	for _, term := range strings.Fields(str) {
		term = strings.TrimFunc(term, func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		term = strings.ToLower(term)

		if len(term) > 0 {
			result = append(result, term)
		}
	}
	return result
}

type logger struct {
	Info  *log.Logger
	Error *log.Logger
}

func NewLogger() *logger {
	return &logger{
		Info:  log.New(os.Stdout, "info: ", 0),
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

type Document struct {
	filepath   string
	terms      []string
	tfidf      StringScoreMap
	tfidfSlice []StringScorePair
}

func tfidfNeedleScore(tfidfSlice []StringScorePair, needle string) float64 {
	type Match struct {
		StringScorePair
		fzy.ScoreResult
	}

	for _, stringScorePair := range tfidfSlice {
		if stringScorePair.String == needle {
			return stringScorePair.Score
		}
	}

	return 0.0
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

	var documents []Document
	{
		var filepaths []string
		root := "./docs/php"
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
			if strings.HasSuffix(filepath, ".html") {
				dataBytes, err := os.ReadFile(filepath)
				if err != nil {
					Logger.Error.Fatalf("Could not read file \"%v\": %v", filepath, err)
				}

				documents = append(documents, Document{
					filepath: filepath,
					terms:    stringToTerms(extractHtmlText(string(dataBytes))),
					tfidf:    make(StringScoreMap),
				})
			}
		}
	}

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
				go func() {
					defer wg.Done()
					for _, term := range doc.terms {
						doc.tfidf[term] += 1.0
					}

					seen := make(map[string]bool)
					term_count := float64(len(doc.terms))
					for _, term := range doc.terms {
						if !seen[term] {
							seen[term] = true
							mutex.Lock()
							numberOfDocumentsContainingTermT[term] += 1.0
							mutex.Unlock()
						}

						doc.tfidf[term] /= term_count
					}
				}()
			}

			wg.Wait()
		}

		// IDF Pass
		{
			documentCount := float64(len(documents))
			for i := range documents {
				wg.Add(1)

				doc := &documents[i]
				go func() {
					defer wg.Done()
					for key := range doc.tfidf {
						idf := documentCount / numberOfDocumentsContainingTermT[key]
						doc.tfidf[key] = doc.tfidf[key] * idf
					}
				}()
			}

			wg.Wait()
		}

		// Convert Hashmaps to array
		{
			for i := range documents {
				wg.Add(1)

				doc := &documents[i]
				go func() {
					defer wg.Done()
					for key, value := range doc.tfidf {
						doc.tfidfSlice = append(doc.tfidfSlice, StringScorePair{String: key, Score: value})
					}
				}()
			}

			wg.Wait()
		}

		// Sort documents by needle
		{
			sort.Slice(documents, func(i, j int) bool {
				scoreA := 0.0
				scoreB := 0.0

				for _, needle := range needles {
					documentA := documents[i]
					documentB := documents[j]

					scoreA += tfidfNeedleScore(documentA.tfidfSlice, needle)
					scoreB += tfidfNeedleScore(documentB.tfidfSlice, needle)
				}

				return scoreB < scoreA
			})
		}

		for _, document := range documents[:5] {
			fmt.Println(document.filepath)
		}
	}
}
