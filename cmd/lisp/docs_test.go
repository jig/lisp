package main

import (
	"reflect"
	"sort"
	"testing"

	"github.com/jig/lisp/docgen"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

// TestAllBuiltinsDocumented is the project-wide guard that no Go builtin
// the binary loads ships without doc metadata (an arglist and a doc
// string), so the LSP, (doc …) and the doc generator describe every
// builtin. It loads exactly the namespaces main loads (via the shared
// libraries list), so a new builtin registered without a call.Doc or a
// docs.go entry fails here.
//
// Lisp-defined functions (MalFunc from the .lisp headers) are exempt —
// their docs come from optional docstrings, not this metadata.
func TestAllBuiltinsDocumented(t *testing.T) {
	ns := env.NewEnv()
	for _, lib := range libraries(nil) {
		if err := lib.load(ns); err != nil {
			t.Fatalf("load %q: %v", lib.name, err)
		}
	}

	var missing []string
	for _, name := range ns.(*env.Env).LocalSymbols() {
		v, err := ns.Get(types.Symbol{Val: name})
		if err != nil {
			continue
		}
		if fn, ok := v.(types.Func); ok && (fn.Doc == "" || fn.Arglist == "") {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("Go builtins missing doc/arglist metadata (%d): %v\n"+
			"Add a call.Doc(env, name, arglist, doc) at registration, or a "+
			"coreDocs entry for core builtins.", len(missing), missing)
	}
}

// TestDocgenLibrariesInSync guards the two copies of the library list:
// the one main loads (libraries, above) and the one docgen documents
// (docgen.standardLibraries). They live in separate packages because
// cmd/lisp is package main and cannot be imported, so this test fails if
// a library is added to one but not the other — which would otherwise
// silently leave LANGUAGE.md missing a whole namespace.
func TestDocgenLibrariesInSync(t *testing.T) {
	ns := env.NewEnv()
	for _, lib := range libraries(nil) {
		if err := lib.load(ns); err != nil {
			t.Fatalf("load %q: %v", lib.name, err)
		}
	}
	mainSymbols := ns.(*env.Env).LocalSymbols()

	docSymbols, err := docgen.StandardSymbols()
	if err != nil {
		t.Fatalf("docgen.StandardSymbols: %v", err)
	}

	sort.Strings(mainSymbols)
	sort.Strings(docSymbols)
	if !reflect.DeepEqual(mainSymbols, docSymbols) {
		t.Errorf("libraries loaded by main differ from those documented by docgen;\n"+
			"keep cmd/lisp's libraries() and docgen.standardLibraries() in sync.\n"+
			"main: %v\ndocgen: %v", mainSymbols, docSymbols)
	}
}
