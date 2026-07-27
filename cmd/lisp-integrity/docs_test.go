package main

import (
	"reflect"
	"sort"
	"testing"

	"github.com/jig/lisp/docgen"
	"github.com/jig/lisp/env"
)

// TestLibrariesMatchDocgen guards this binary's libraries list against
// drifting from docgen's (which cmd/lisp's own sync test ties to the
// lisp binary): both must build exactly the same environment.
func TestLibrariesMatchDocgen(t *testing.T) {
	ns := env.NewEnv()
	for _, lib := range libraries(nil) {
		if err := lib.load(ns); err != nil {
			t.Fatalf("load %q: %v", lib.name, err)
		}
	}
	ours := ns.(*env.Env).LocalSymbols()
	sort.Strings(ours)

	standard, err := docgen.StandardSymbols()
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(standard)

	if !reflect.DeepEqual(ours, standard) {
		t.Fatalf("lisp-integrity loads a different environment than the lisp binary/docgen")
	}
}
