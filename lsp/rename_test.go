package lsp

import (
	"testing"
)

// TestServer_PrepareRename_WholeHyphenatedSymbol checks that prepareRename
// returns the range of the entire hyphenated symbol, not a fragment.
func TestServer_PrepareRename_WholeHyphenatedSymbol(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///rename.lisp"
	didOpen(t, client, uri, "(defn my-func [x] x)\n(my-func 1)\n")

	// Cursor on the "f" of my-func (line 0, char 9).
	send(t, client, 10, "textDocument/prepareRename", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 9},
	})
	resp := readUntil(t, client, response(10))
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a range from prepareRename, got %v", resp["result"])
	}
	start := result["start"].(map[string]interface{})
	end := result["end"].(map[string]interface{})
	if int(start["character"].(float64)) != 6 || int(end["character"].(float64)) != 13 {
		t.Fatalf("expected the whole symbol range [6,13), got [%v,%v)", start["character"], end["character"])
	}
}

// TestServer_PrepareRename_RefusesUndefinedSymbol checks that a symbol not
// defined in the document (a builtin here) cannot be renamed.
func TestServer_PrepareRename_RefusesUndefinedSymbol(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///refuse.lisp"
	didOpen(t, client, uri, "(defn foo [x] x)\n(+ 1 2)\n")

	// Cursor on the "+" (line 1, char 1) — a builtin, not a local def.
	send(t, client, 11, "textDocument/prepareRename", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 1},
	})
	resp := readUntil(t, client, response(11))
	if resp["result"] != nil {
		t.Fatalf("expected null (cannot rename a builtin), got %v", resp["result"])
	}
}

// TestServer_Rename renames a hyphenated definition and all its references.
func TestServer_Rename(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///do-rename.lisp"
	didOpen(t, client, uri, "(defn my-func [x] x)\n(my-func 1)\n")

	send(t, client, 12, "textDocument/rename", RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 9},
		NewName:      "renamed",
	})
	resp := readUntil(t, client, response(12))
	edits := renameEdits(t, resp, uri)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (def + call), got %d: %v", len(edits), edits)
	}
	for _, e := range edits {
		if e["newText"] != "renamed" {
			t.Errorf("expected newText 'renamed', got %v", e["newText"])
		}
	}
}

// TestServer_Rename_SkipsStringsAndComments verifies textual occurrences
// inside strings and comments are not rewritten.
func TestServer_Rename_SkipsStringsAndComments(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///skip.lisp"
	src := "(defn foo [x] x)\n" + // 0: definition
		"(foo 1)\n" + // 1: call
		";; foo in a comment\n" + // 2: comment
		"(println \"foo bar\")\n" // 3: string
	didOpen(t, client, uri, src)

	send(t, client, 13, "textDocument/rename", RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 1, Character: 2},
		NewName:      "baz",
	})
	resp := readUntil(t, client, response(13))
	edits := renameEdits(t, resp, uri)
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits (def + call only), got %d: %v", len(edits), edits)
	}
	for _, e := range edits {
		rng := e["range"].(map[string]interface{})
		line := int(rng["start"].(map[string]interface{})["line"].(float64))
		if line != 0 && line != 1 {
			t.Errorf("edit on unexpected line %d (should skip comment/string)", line)
		}
	}
}

// TestServer_Rename_RejectsInvalidNewName refuses a new name containing a
// delimiter.
func TestServer_Rename_RejectsInvalidNewName(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///bad-name.lisp"
	didOpen(t, client, uri, "(defn foo [x] x)\n")

	send(t, client, 14, "textDocument/rename", RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 7},
		NewName:      "bad name",
	})
	resp := readUntil(t, client, response(14))
	if resp["error"] == nil {
		t.Fatalf("expected an error for an invalid new name, got %v", resp)
	}
}

// renameEdits extracts the edit list for uri from a rename response.
func renameEdits(t *testing.T, resp map[string]interface{}, uri string) []map[string]interface{} {
	t.Helper()
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a WorkspaceEdit result, got %v", resp["result"])
	}
	changes, ok := result["changes"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected changes in the WorkspaceEdit, got %v", result)
	}
	raw, ok := changes[uri].([]interface{})
	if !ok {
		t.Fatalf("expected edits for %s, got %v", uri, changes)
	}
	out := make([]map[string]interface{}, len(raw))
	for i, e := range raw {
		out[i] = e.(map[string]interface{})
	}
	return out
}
