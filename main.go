package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func iterateTerms(str string, termFunc func(term string)) uint64 {
	var result uint64
	for _, term := range strings.Fields(str) {
		term = strings.TrimFunc(term, func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		term = strings.ToLower(term)

		result += 1
		if len(term) > 0 {
			termFunc(term)
		}
	}
	return result
}

type StringScoreMap map[string]float64

type TermScoreTFIDF struct {
	fzy.ScoreResult
	Tfidf float64
}

type Document struct {
	Name            string
	TermScoreTFIDFS []TermScoreTFIDF
}

func TermScoreTFIDFLess(a, b TermScoreTFIDF) bool {
	if a.ScoreResult.Score-b.ScoreResult.Score <= 2 {
		return a.Tfidf < b.Tfidf
	}
	return fzy.ScoreResultLess(a.ScoreResult, b.ScoreResult)
}

func SortedTermScoreTFIDFByNeedle(tfidf StringScoreMap, needle string) []TermScoreTFIDF {
	result := make([]TermScoreTFIDF, len(tfidf))

	var result_idx uint
	for key, value := range tfidf {
		result[result_idx] = TermScoreTFIDF{fzy.Score(key, needle), value}
		result_idx += 1
	}

	sort.Slice(result, func(i, j int) bool {
		return TermScoreTFIDFLess(result[j], result[i])
	})

	return result
}

var (
	needle string
)

func main() {
	// Parse CLI Args
	{
		flag.StringVar(&needle, "needle", "", "")
		flag.Parse()

		if len(needle) == 0 {
			fmt.Println("ERROR: No needle was provided")
			os.Exit(1)
		}
	}

	var filepaths []string
	{
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
			fmt.Printf("Error walking path: \"%v\": %v\n", root, err)
		}
	}

	tfidfs := make(map[string]StringScoreMap)
	{
		type TfidfResult struct {
			filepath string
			StringScoreMap
		}

		channel := make(chan TfidfResult)
		for _, filepath := range filepaths {
			go func() {
				data, err := os.ReadFile(filepath)
				if err != nil {
					fmt.Printf("Error: Could not read file \"%v\": %v", filepath, err)
					return
				}

				txt := extractHtmlText(string(data))

				// Calculate TF
				tf := make(StringScoreMap)
				{
					term_count := iterateTerms(txt, func(term string) {
						tf[term] += 1.0
					})

					for key := range tf {
						tf[key] /= float64(term_count)
					}
				}

				// Calculate IDF
				idf := make(StringScoreMap)
				{
					for _, filepath := range filepaths {
						data, err := os.ReadFile(filepath)
						if err != nil {
							fmt.Printf("Error: Could not read file \"%v\": %v", filepath, err)
							continue
						}

						seen := make(map[string]bool)
						txt := extractHtmlText(string(data))
						iterateTerms(txt, func(term string) {
							if !seen[term] {
								seen[term] = true
								idf[term] += 1.0
							}
						})
					}

					for key := range idf {
						idf[key] = math.Log(float64(len(filepaths)) / idf[key])
					}
				}

				// Calculate TFIDF
				tfidf := make(StringScoreMap)
				{
					for key := range tf {
						tfidf[key] = tf[key] * idf[key]
					}
				}

				channel <- TfidfResult{filepath, tfidf}
			}()
		}

		// TODO: convert tfidfs to an array instead
		// TODO: concurrent fzy
		for result := range channel {
			tfidfs[result.filepath] = result.StringScoreMap
		}
	}

	documents := make([]Document, len(tfidfs))
	{
		var document_idx uint
		for key, value := range tfidfs {
			scoreTFIDF := SortedTermScoreTFIDFByNeedle(value, needle)
			document := Document{key, scoreTFIDF}
			documents[document_idx] = document
			document_idx += 1
		}

		// Sort Documents
		{
			sort.Slice(documents, func(i, j int) bool {
				result := true
				for needle_idx, _ := range strings.Split(needle, " ") {
					result = result && TermScoreTFIDFLess(documents[j].TermScoreTFIDFS[needle_idx], documents[i].TermScoreTFIDFS[needle_idx])
				}
				return result
			})
		}
	}

	for document_idx, document := range documents {
		if document_idx > 5 {
			break
		}

		fmt.Println(document.Name)
	}
}
