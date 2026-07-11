package lsp

import (
	"testing"
)

// TestSemanticTokens_Classification checks each symbol is classified with
// the expected token type: special forms as keywords, the defn name as a
// function declaration, parameters (binding and use) as parameters, and a
// builtin call head as a function.
func TestSemanticTokens_Classification(t *testing.T) {
	src := "(defn my-func [x] (+ x 1))\n"
	a := analyseDocument("t.lisp", src)
	toks := a.semanticTokens(src)

	type tk struct {
		typ  int
		mods int
	}
	got := map[int]tk{} // keyed by start char (all on line 0)
	for _, s := range toks {
		if s.rng.Start.Line != 0 {
			t.Fatalf("unexpected token on line %d", s.rng.Start.Line)
		}
		got[s.rng.Start.Character] = tk{s.typ, s.mods}
	}

	want := map[int]tk{
		1:  {tokKeyword, 0},                    // defn
		6:  {tokFunction, tokModDeclaration},   // my-func (declaration)
		15: {tokParameter, tokModDeclaration},  // x (binding)
		19: {tokFunction, 0},                   // + (builtin call head)
		21: {tokParameter, 0},                  // x (use)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d: %v", len(want), len(got), got)
	}
	for char, w := range want {
		g, ok := got[char]
		if !ok {
			t.Errorf("no token at char %d (want %v)", char, w)
			continue
		}
		if g != w {
			t.Errorf("token at char %d: got %v, want %v", char, g, w)
		}
	}
}

// TestSemanticTokens_MacroAndDef checks defmacro names and def names are
// distinguished, and a macro call head reads as a macro.
func TestSemanticTokens_MacroAndDef(t *testing.T) {
	src := "(def answer 42)\n(defmacro twice [x] x)\n(twice answer)\n"
	a := analyseDocument("t.lisp", src)
	toks := a.semanticTokens(src)

	byPos := map[[2]int]int{}
	for _, s := range toks {
		byPos[[2]int{s.rng.Start.Line, s.rng.Start.Character}] = s.typ
	}
	// "answer" def name (line 0, char 5) is a variable.
	if byPos[[2]int{0, 5}] != tokVariable {
		t.Errorf("def name should be variable, got %v", byPos[[2]int{0, 5}])
	}
	// "twice" defmacro name (line 1, char 10) is a macro declaration.
	if byPos[[2]int{1, 10}] != tokMacro {
		t.Errorf("defmacro name should be macro, got %v", byPos[[2]int{1, 10}])
	}
	// "twice" call head (line 2, char 1) reads as a macro.
	if byPos[[2]int{2, 1}] != tokMacro {
		t.Errorf("macro call head should be macro, got %v", byPos[[2]int{2, 1}])
	}
	// "answer" use (line 2, char 7) reads as a variable.
	if byPos[[2]int{2, 7}] != tokVariable {
		t.Errorf("variable use should be variable, got %v", byPos[[2]int{2, 7}])
	}
}

// TestSemanticTokens_WireFormat checks the delta-encoded response is
// well-formed (a multiple of five integers) and non-empty.
func TestSemanticTokens_WireFormat(t *testing.T) {
	client, stop := startSession(t)
	defer stop()

	uri := "file:///sem.lisp"
	didOpen(t, client, uri, "(defn f [x] (+ x 1))\n")

	send(t, client, 40, "textDocument/semanticTokens/full", SemanticTokensParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	resp := readUntil(t, client, response(40))
	data := resp["result"].(map[string]interface{})["data"].([]interface{})
	if len(data) == 0 || len(data)%5 != 0 {
		t.Fatalf("expected a non-empty multiple-of-5 token array, got %d ints", len(data))
	}
}
