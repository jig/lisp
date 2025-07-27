package main

import (
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/jig/lisp/doc"
)

type FunctionInfo struct {
	Name        string
	Description string
	Library     string
}

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

	var allFunctions []FunctionInfo

	// Collect all functions from all libraries
	for _, file := range files {
		data, err := doc.AllJSONFiles.ReadFile(file.Name())
		if err != nil {
			log.Fatalf("Failed to read file %s: %v", file.Name(), err)
			return
		}

		var docsLib doc.Library
		if err := json.Unmarshal(data, &docsLib); err != nil {
			log.Fatalf("Failed to decode documentation for %s: %v", file.Name(), err)
		}

		// Add all symbols from this library
		for symbolName, symbol := range docsLib.Symbols {
			allFunctions = append(allFunctions, FunctionInfo{
				Name:        symbolName,
				Description: symbol.Description,
				Library:     docsLib.Name,
			})
		}
	}

	// Sort functions alphabetically by name
	sort.Slice(allFunctions, func(i, j int) bool {
		return strings.ToLower(allFunctions[i].Name) < strings.ToLower(allFunctions[j].Name)
	})

	// Generate markdown content
	var markdown strings.Builder
	markdown.WriteString("# All Functions Reference\n\n")
	markdown.WriteString("Complete alphabetical list of all functions across all libraries.\n\n")
	markdown.WriteString("---\n\n")

	for _, fn := range allFunctions {
		markdown.WriteString("**")
		markdown.WriteString(escapeMarkdown(fn.Name))
		markdown.WriteString("**: ")
		markdown.WriteString(fn.Description)
		markdown.WriteString("\n\n")
	}

	// Write to single markdown file
	outputFileName := "all-functions.md"
	if err := os.WriteFile(outputFileName, []byte(markdown.String()), 0644); err != nil {
		log.Fatalf("Failed to write markdown file %s: %v", outputFileName, err)
	}

	log.Printf("Generated %s with %d functions from %d libraries", outputFileName, len(allFunctions), len(files))
}
