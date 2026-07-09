package core

import (
	"sort"
	"testing"

	"github.com/jig/lisp/env"
	. "github.com/jig/lisp/types"
)

// TestCoreBuiltinsAllDocumented is the reverse of TestCoreDocsAreHonest:
// it asserts every Go builtin registered by core Load/LoadInput carries
// doc metadata, so a new builtin added without a docs.go entry (or a
// call.Doc) is caught instead of silently reaching the LSP undocumented.
func TestCoreBuiltinsAllDocumented(t *testing.T) {
	ns := env.NewEnv()
	Load(ns)
	LoadInput(ns)
	var missing []string
	for _, name := range ns.(*env.Env).LocalSymbols() {
		v, err := ns.Get(Symbol{Val: name})
		if err != nil {
			continue
		}
		if fn, ok := v.(Func); ok && (fn.Doc == "" || fn.Arglist == "") {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("core Go builtins missing doc/arglist (add a coreDocs entry): %v", missing)
	}
}

// TestCoreDocsAreHonest asserts every documented core builtin is really
// registered by Load as a Go function, so the doc table cannot drift
// from the actual builtins.
func TestCoreDocsAreHonest(t *testing.T) {
	ns := env.NewEnv()
	Load(ns)
	LoadInput(ns)
	for _, d := range coreDocs {
		v, err := ns.Get(Symbol{Val: d.name})
		if err != nil {
			t.Errorf("%q documented but not registered by core.Load", d.name)
			continue
		}
		fn, ok := v.(Func)
		if !ok {
			t.Errorf("%q documented as a Go builtin but is %T", d.name, v)
			continue
		}
		if fn.Doc != d.doc || fn.Arglist != d.arglist {
			t.Errorf("%q doc fields mismatch: Doc=%q Arglist=%q", d.name, fn.Doc, fn.Arglist)
		}
		// Documentation must NOT leak into meta (kanaka/mal keeps native
		// function metadata nil).
		if fn.Meta != nil {
			t.Errorf("%q meta should stay nil, got %v", d.name, fn.Meta)
		}
	}
}
