package core

import (
	"os"
	"testing"
)

// TestReadLineEOF verifies readline returns nil at end of input (Ctrl-D),
// so a REPL loop can tell EOF apart from an empty line and terminate.
func TestReadLineEOF(t *testing.T) {
	empty, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = empty.Close() }()

	old := os.Stdin
	os.Stdin = empty // empty file → immediate EOF
	defer func() { os.Stdin = old }()

	got, err := readLine("")
	if err != nil {
		t.Fatalf("readLine at EOF: %v", err)
	}
	if got != nil {
		t.Errorf("readLine at EOF = %v, want nil", got)
	}
}
