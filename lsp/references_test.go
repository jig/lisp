package lsp

import (
	"sort"
	"testing"
)

// referenceLines returns the sorted 0-based start lines of a references
// response.
func referenceLines(t *testing.T, resp map[string]interface{}) []int {
	t.Helper()
	raw, ok := resp["result"].([]interface{})
	if !ok {
		t.Fatalf("expected a locations array, got %v", resp["result"])
	}
	lines := make([]int, 0, len(raw))
	for _, loc := range raw {
		rng := loc.(map[string]interface{})["range"].(map[string]interface{})
		start := rng["start"].(map[string]interface{})
		lines = append(lines, int(start["line"].(float64)))
	}
	sort.Ints(lines)
	return lines
}

func TestServer_References_IncludesDeclaration(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///refs.lisp"
	didOpen(t, client, uri, "(defn my-func [x] x)\n(my-func 1)\n(my-func 2)\n")

	send(t, client, 20, "textDocument/references", ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 9},
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	resp := readUntil(t, client, response(20))
	got := referenceLines(t, resp)
	want := []int{0, 1, 2} // definition + two calls
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("expected references on lines %v, got %v", want, got)
	}
}

func TestServer_References_ExcludesDeclaration(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///refs2.lisp"
	didOpen(t, client, uri, "(defn my-func [x] x)\n(my-func 1)\n(my-func 2)\n")

	send(t, client, 21, "textDocument/references", ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
		Context:      ReferenceContext{IncludeDeclaration: false},
	})
	resp := readUntil(t, client, response(21))
	got := referenceLines(t, resp)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected only the two call sites (lines 1,2), got %v", got)
	}
}

func TestServer_References_SkipsStringsAndComments(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///refs3.lisp"
	src := "(defn foo [x] x)\n" + // 0: def
		"(foo 1)\n" + // 1: call
		";; foo mentioned\n" + // 2: comment
		"(println \"foo\")\n" // 3: string
	didOpen(t, client, uri, src)

	send(t, client, 22, "textDocument/references", ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	resp := readUntil(t, client, response(22))
	got := referenceLines(t, resp)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("expected references only on lines 0,1 (def+call), got %v", got)
	}
}
