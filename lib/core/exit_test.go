package core

import (
	"testing"
)

// TestExit checks the exit builtin through the osExit indirection, so the
// test observes the requested status instead of terminating the test binary.
func TestExit(t *testing.T) {
	orig := osExit
	defer func() { osExit = orig }()

	var got int
	called := false
	osExit = func(code int) { got, called = code, true }

	// (exit 3) requests status 3
	if _, err := exit(3); err != nil {
		t.Fatalf("exit(3): unexpected error %v", err)
	}
	if !called || got != 3 {
		t.Fatalf("exit(3): called=%v got=%d, want status 3", called, got)
	}

	// (exit) defaults to status 0
	called, got = false, -1
	if _, err := exit(); err != nil {
		t.Fatalf("exit(): unexpected error %v", err)
	}
	if !called || got != 0 {
		t.Fatalf("exit(): called=%v got=%d, want status 0", called, got)
	}

	// (exit "x") is a type error and must not terminate
	called = false
	if _, err := exit("x"); err == nil {
		t.Fatal(`exit("x"): expected an error for a non-integer status`)
	}
	if called {
		t.Fatal(`exit("x"): must not terminate on a bad argument`)
	}
}
