package txt

import (
	"fmt"
	"os"
	"strings"

	"github.com/cheggaaa/go-poppler"
	"golang.org/x/net/html"
)

func FromHtml(filepath string) (string, error) {
	htmlData, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

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
				stringBuilder.WriteString(fmt.Sprintf(" %v", txt))
			}
		}
	}

	return stringBuilder.String(), nil
}

func FromPdf(filepath string) (string, error) {
	doc, err := poppler.Open(filepath)
	if err != nil {
		return "", err
	}

	numPages := doc.GetNPages()
	var texts []string
	for i := 0; i < numPages; i += 1 {
		texts = append(texts, doc.GetPage(i).Text())
	}

	return strings.Join(texts, " "), nil
}

func FromFilepath(filepath string) (string, error) {
	var result string
	var err error

	var extension string
	if parts := strings.Split(filepath, "."); len(parts) > 0 {
		extension = parts[len(parts)-1]
	}

	switch extension {
	case "html", "htm", "xhtml", "xml":
		result, err = FromHtml(filepath)
	case "pdf":
		result, err = FromPdf(filepath)
	case "md", "txt":
		var bytes []byte
		bytes, err = os.ReadFile(filepath)
		result = string(bytes)
	default:
		err = fmt.Errorf("filetype \"%v\" is not supported.", extension)
	}

	return result, err
}
