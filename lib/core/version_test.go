package core

import (
	"os"
	"runtime/debug"
	"strings"
	"testing"
)

// TestModuleVersion checks that a module's version is found whether it is
// the main module or a dependency, and is empty when absent.
func TestModuleVersion(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Path: "example.com/main", Version: "v1.0.0"},
		Deps: []*debug.Module{
			{Path: "example.com/dep", Version: "v2.3.4"},
		},
	}
	if got := moduleVersion(bi, "example.com/main"); got != "v1.0.0" {
		t.Errorf("main module: got %q, want v1.0.0", got)
	}
	if got := moduleVersion(bi, "example.com/dep"); got != "v2.3.4" {
		t.Errorf("dependency: got %q, want v2.3.4", got)
	}
	if got := moduleVersion(bi, "example.com/missing"); got != "" {
		t.Errorf("missing module: got %q, want empty", got)
	}
}

// TestVersions checks the shared helper behind --version and (version).
// Only the main module and the Go version are asserted: a `go test`
// binary's build info carries no Deps, so jig/scanner (a dependency)
// resolves only in a real build — that path is covered by
// TestModuleVersion instead.
func TestVersions(t *testing.T) {
	lispVer, _, goVer := Versions()
	if lispVer == "" {
		t.Error("lisp version is empty; the main module should resolve")
	}
	if !strings.HasPrefix(goVer, "go") {
		t.Errorf("go version = %q, want a value starting with \"go\"", goVer)
	}
}

// TestVersionBuiltin checks the (version) builtin returns the expected
// top-level structure.
func TestVersionBuiltin(t *testing.T) {
	v, err := version()
	if err != nil {
		t.Fatalf("version(): %v", err)
	}
	for _, k := range []string{"ʞgo-version", "ʞbuild", "ʞdependencies"} {
		if _, ok := v.Val[k]; !ok {
			t.Errorf("version() is missing key %q", k)
		}
	}
}

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
