package docgen_test

import (
	"os"
	"testing"

	"github.com/jig/lisp/docgen"
)

const langDoc = "../LANGUAGE.md"

// TestLanguageDocUpToDate fails when the generated builtin reference in
// LANGUAGE.md has drifted from the interpreter — i.e. a builtin was
// added, removed, or its doc/arglist changed without regenerating the
// document. Regenerate with `go generate ./...`.
func TestLanguageDocUpToDate(t *testing.T) {
	current, err := os.ReadFile(langDoc)
	if err != nil {
		t.Fatalf("read %s: %v", langDoc, err)
	}
	want, err := docgen.Splice(string(current))
	if err != nil {
		t.Fatalf("splice: %v", err)
	}
	if want != string(current) {
		t.Errorf("%s is out of date; run `go generate ./...` to regenerate the builtin reference", langDoc)
	}
}

// TestMarkdownRenders is a smoke test: the reference must generate
// without error and be non-trivial.
func TestMarkdownRenders(t *testing.T) {
	md, err := docgen.Markdown()
	if err != nil {
		t.Fatal(err)
	}
	if len(md) < 1000 {
		t.Fatalf("generated reference suspiciously short (%d bytes)", len(md))
	}
}
