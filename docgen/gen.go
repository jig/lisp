//go:build ignore

// Command gen regenerates the builtin-reference block of LANGUAGE.md.
// It is not part of the normal build; run it with:
//
//	go generate ./...
//
// or directly:
//
//	go run ./docgen/gen.go -o LANGUAGE.md
package main

import (
	"flag"
	"log"

	"github.com/jig/lisp/docgen"
)

func main() {
	out := flag.String("o", "../LANGUAGE.md", "path of the document to update in place")
	flag.Parse()

	changed, err := docgen.UpdateFile(*out)
	if err != nil {
		log.Fatalf("docgen: %v", err)
	}
	if changed {
		log.Printf("docgen: updated %s", *out)
	} else {
		log.Printf("docgen: %s already up to date", *out)
	}
}
