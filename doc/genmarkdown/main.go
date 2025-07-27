package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/jig/lisp/doc"
)

// escapeMarkdown escapes special Markdown characters in symbol names
func escapeMarkdown(s string) string {
	// Escape asterisks, underscores, and backticks that have special meaning in Markdown
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}

func main() {
	files, err := doc.AllJSONFiles.ReadDir(".")
	if err != nil {
		log.Fatalf("Failed to read embedded files: %v", err)
	}

	totalSymbols := 0

	for _, file := range files {
		data, err := doc.AllJSONFiles.ReadFile(file.Name())
		if err != nil {
			log.Fatalf("Failed to read file %s: %v", file.Name(), err)
			return
		}

		// Test decoding of the core documentation
		var docsLib doc.Library
		if err := json.Unmarshal(data, &docsLib); err != nil {
			log.Fatalf("Failed to decode core documentation: %v", err)
		}
		totalSymbols += len(docsLib.Symbols)

		// Generate complete markdown documentation for the library
		markdown := libraryJSONToMarkdown(docsLib)

		// Write markdown to file
		outputFileName := file.Name()[:len(file.Name())-5] + ".md"
		if err := os.WriteFile(outputFileName, []byte(markdown), 0644); err != nil {
			log.Fatalf("Failed to write markdown file %s: %v", outputFileName, err)
		}
	}
}

func libraryJSONToMarkdown(lib doc.Library) string {
	const tmpl = `# Library: {{.Name}}

**Library Version:** {{.Version}}

{{.Description}}

## Library Summary

This library contains **{{.SymbolCount}} symbols** organized in the following categories:

{{range $category, $count := .Categories}}
- **{{$category}}:** {{$count}} symbols
{{end}}

## Library Symbol Types

{{range $symbolType, $count := .SymbolTypes}}
- **{{$symbolType}}:** {{$count}} symbols
{{end}}

---

## Library Functions and Symbols

{{.SymbolsMarkdown}}
`

	// Analyze symbols to create summary
	categories := make(map[string]int)
	symbolTypes := make(map[string]int)
	symbolsMarkdown := ""

	for symbolName, symbol := range lib.Symbols {
		// Count categories
		if category, ok := symbol.Metadata["category"].(string); ok {
			categories[category]++
		} else {
			categories["uncategorized"]++
		}

		// Count symbol types
		if symbolType, ok := symbol.Metadata["symbol-type"].(string); ok {
			symbolTypes[symbolType]++
		} else {
			symbolTypes["unknown"]++
		}

		// Generate markdown for each symbol
		symbolsMarkdown += symbolJSONToMarkdown(symbolName, symbol) + "\n\n"
	}

	t, err := template.New("library").Parse(tmpl)
	if err != nil {
		log.Printf("Error parsing library template: %v", err)
		return ""
	}

	data := struct {
		Name            string
		Version         string
		Description     string
		SymbolCount     int
		Categories      map[string]int
		SymbolTypes     map[string]int
		SymbolsMarkdown string
	}{
		Name:            lib.Name,
		Version:         lib.Version,
		Description:     lib.Description,
		SymbolCount:     len(lib.Symbols),
		Categories:      categories,
		SymbolTypes:     symbolTypes,
		SymbolsMarkdown: symbolsMarkdown,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		log.Printf("Error executing library template: %v", err)
		return ""
	}

	return buf.String()
}

func symbolJSONToMarkdown(symbolName string, symbol doc.Symbols) string {
	const tmpl = `
## {{.Name}}

**Description:** {{.Symbol.Description}}

{{if .Symbol.Args}}

### Arguments

{{range .Symbol.Args}}
- **{{.Name}}** ({{.Type}}){{if .Variadic}} *variadic*{{end}}: {{.Description}}
{{end}}
{{end}}

### Returns

**Type:** {{.Symbol.Returns.Type}}

**Description:** {{.Symbol.Returns.Description}}

{{if .Symbol.Errors}}
### Errors
{{range .Symbol.Errors}}
- {{.}}
{{end}}
{{end}}

{{if .Symbol.Examples}}
### Examples
{{range .Symbol.Examples}}
` + "```clojure" + `
{{.Input}}
;; => {{.Output}}
` + "```" + `
{{end}}
{{end}}

{{if .Symbol.Metadata}}
### Metadata
{{range $key, $value := .Symbol.Metadata}}
- **{{$key}}:** {{$value}}
{{end}}
{{end}}
`

	t, err := template.New("symbol").Parse(tmpl)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		return ""
	}

	data := struct {
		Name   string
		Symbol doc.Symbols
	}{
		Name:   escapeMarkdown(symbolName),
		Symbol: symbol,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		log.Printf("Error executing template: %v", err)
		return ""
	}

	return buf.String()
}
