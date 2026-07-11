package command

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jig/lisp/types"
)

// TestRunScript_FileVar asserts *FILE* is bound to the absolute path
// of the script being run, on both runScript paths (classic load-file
// and the preamble path).
func TestRunScript_FileVar(t *testing.T) {
	script := writeScript(t, "self.lisp", "*FILE*\n")
	abs, err := filepath.Abs(script)
	if err != nil {
		t.Fatal(err)
	}

	ns := preambleTestEnv(t)
	out, err := runScript(context.Background(), ns, script, nil, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if out != `"`+abs+`"` {
		t.Errorf("classic path: *FILE* = %v, want %q", out, abs)
	}

	// Preamble path (placeholder present forces the non-load-file path).
	script2 := writeScript(t, "self2.lisp", ";; $A 1\n(str *FILE*)\n")
	abs2, err := filepath.Abs(script2)
	if err != nil {
		t.Fatal(err)
	}
	ns2 := preambleTestEnv(t)
	out, err = runScript(context.Background(), ns2, script2, nil, types.NewCursorHere(script2, -3, 1))
	if err != nil {
		t.Fatalf("runScript preamble path: %v", err)
	}
	if out != `"`+abs2+`"` {
		t.Errorf("preamble path: *FILE* = %v, want %q", out, abs2)
	}
}

// TestRunScript_FileVarSelfRead asserts the motivating use case: a
// script reading its own source through *FILE*.
func TestRunScript_FileVarSelfRead(t *testing.T) {
	script := writeScript(t, "selfread.lisp", `(starts-with? (slurp *FILE*) "(starts-with?")`+"\n")
	ns := preambleTestEnv(t)
	out, err := runScript(context.Background(), ns, script, nil, types.NewCursorHere(script, -3, 1))
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if out != "true" {
		t.Errorf("script could not read itself via *FILE*: got %v", out)
	}
}
