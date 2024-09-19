package main

import (
	"fmt"
	"golang.org/x/net/html"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var filepaths []string
	{
		root := "./docs/test"
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

	for _, filepath := range filepaths {
		data, err := os.ReadFile(filepath)
		if err != nil {
			fmt.Printf("Error: Could not read file \"%v\": %v", filepath, err)
			continue
		}

		tokenizer := html.NewTokenizer(strings.NewReader(string(data)))
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
				}
			}
		}
	}
}
