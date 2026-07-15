package core

import (
	"os"
	"sync"
	"testing"
)

// withStdin runs fn with os.Stdin fed from content and the shared
// line-scanner reset, so the read-* builtins observe exactly this input.
func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	// Reset the shared scanner to fresh state (a sync.Once cannot be
	// copied, so restore by re-zeroing rather than saving the old value —
	// no other test relies on prior scanner state).
	stdinScanner = nil
	stdinScannerOnce = sync.Once{}
	t.Cleanup(func() {
		os.Stdin = orig
		stdinScanner = nil
		stdinScannerOnce = sync.Once{}
		_ = r.Close()
	})
	fn()
}

// TestReadPasswordPipedFallback covers the non-terminal path: with stdin
// piped there is no echo to suppress, so it reads a line like readline.
// (The echo-off terminal path needs a pty and is exercised manually.)
func TestReadPasswordPipedFallback(t *testing.T) {
	withStdin(t, "hunter2\nsecond\n", func() {
		got, err := readPassword("Password: ")
		if err != nil {
			t.Fatal(err)
		}
		if got != "hunter2" {
			t.Fatalf("got %#v, want \"hunter2\"", got)
		}
	})

	// End of input returns nil, like readline, so callers can detect EOF.
	withStdin(t, "", func() {
		got, err := readPassword("Password: ")
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("on EOF got %#v, want nil", got)
		}
	})
}
