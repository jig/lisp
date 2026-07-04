package core

import (
	"testing"

	"github.com/jig/lisp/env"
	. "github.com/jig/lisp/types"
)

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
