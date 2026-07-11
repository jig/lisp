package lsp

import (
	"sort"
	"testing"
)

// editLines returns the sorted set of distinct start lines of a rename
// response's edits.
func editLines(t *testing.T, resp map[string]interface{}, uri string) []int {
	t.Helper()
	seen := map[int]bool{}
	for _, e := range renameEdits(t, resp, uri) {
		rng := e["range"].(map[string]interface{})
		seen[int(rng["start"].(map[string]interface{})["line"].(float64))] = true
	}
	out := make([]int, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Ints(out)
	return out
}

// TestServer_Rename_LocalIsScoped renames a parameter and checks only its
// own function's occurrences change — not a same-named parameter in a
// sibling function.
func TestServer_Rename_LocalIsScoped(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///local.lisp"
	didOpen(t, client, uri, "(defn f [x] (+ x 1))\n(defn g [x] (* x 2))\n")

	// Cursor on the use of x inside f (line 0, char 15).
	send(t, client, 30, "textDocument/rename", RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 15},
		NewName:      "y",
	})
	resp := readUntil(t, client, response(30))
	edits := renameEdits(t, resp, uri)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (param + use in f only), got %d: %v", len(edits), edits)
	}
	if lines := editLines(t, resp, uri); len(lines) != 1 || lines[0] != 0 {
		t.Fatalf("expected all edits on line 0 (f), got lines %v", lines)
	}
}

// TestServer_Rename_TopLevelExcludesShadow renames a top-level def and
// checks a shadowing local of the same name is left untouched.
func TestServer_Rename_TopLevelExcludesShadow(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///shadow.lisp"
	didOpen(t, client, uri, "(def x 10)\n(defn f [x] x)\n(println x)\n")

	// Cursor on the top-level def name x (line 0, char 5).
	send(t, client, 31, "textDocument/rename", RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 5},
		NewName:      "z",
	})
	resp := readUntil(t, client, response(31))
	lines := editLines(t, resp, uri)
	// Def site (line 0) and the free use (line 2); the shadowed uses on
	// line 1 must be excluded.
	if len(lines) != 2 || lines[0] != 0 || lines[1] != 2 {
		t.Fatalf("expected edits on lines [0 2] (shadow on line 1 excluded), got %v", lines)
	}
}

// TestServer_References_LocalIsScoped checks find-references on a local
// only lists that binding's occurrences.
func TestServer_References_LocalIsScoped(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///local-refs.lisp"
	didOpen(t, client, uri, "(defn f [x] (+ x 1))\n(defn g [x] (* x 2))\n")

	send(t, client, 32, "textDocument/references", ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 15},
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	resp := readUntil(t, client, response(32))
	lines := referenceLines(t, resp)
	if len(lines) != 2 || lines[0] != 0 || lines[1] != 0 {
		t.Fatalf("expected both references on line 0 (f's x), got %v", lines)
	}
}

// TestServer_PrepareRename_AllowsLocal checks a local binding (not a
// top-level def) is now renameable, and returns the whole-symbol range.
func TestServer_PrepareRename_AllowsLocal(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///allow-local.lisp"
	didOpen(t, client, uri, "(defn f [count] (+ count 1))\n")

	// Cursor inside the local "count" use.
	send(t, client, 33, "textDocument/prepareRename", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 20},
	})
	resp := readUntil(t, client, response(33))
	if _, ok := resp["result"].(map[string]interface{}); !ok {
		t.Fatalf("expected a range (local is renameable), got %v", resp["result"])
	}
}
