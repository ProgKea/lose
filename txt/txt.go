package txt

import (
	"fmt"
	"os"
	"strings"

	"code.sajari.com/docconv/v2"
	"golang.org/x/net/html"
)

func FromHtml(htmlData string) string {
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

	return stringBuilder.String()
}

func FromExtension(extension, data string) string {
	switch extension {
	case "html", "htm", "xhtml", "xml":
		return FromHtml(data)
	default:
		fmt.Fprintf(os.Stderr, "warn: have no function for extension \"%v\" using unparsed data.\n", extension)
	}

	return data
}

func FromFilepath(filepath string) string {
	var content string
	res, err := docconv.ConvertPath(filepath)
	if err != nil {
		bytes, err := os.ReadFile(filepath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not read file \"%v\", exiting. (%v).\n", filepath, err)
			os.Exit(1)
		}

		{
			var extension string
			if parts := strings.Split(filepath, "."); len(parts) > 0 {
				extension = parts[len(parts)-1]
			}
			content = FromExtension(extension, string(bytes))
		}
	} else {
		content = string(res.Body)
	}

	return content
}
