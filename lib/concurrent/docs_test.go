package concurrent

import (
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

// TestAtomDocs asserts the atom builtins carry documentation metadata
// after Load, so the LSP and (doc …) describe them.
func TestAtomDocs(t *testing.T) {
	ns := env.NewEnv()
	Load(ns)
	for _, name := range []string{"atom", "atom?", "reset!", "swap!"} {
		v, err := ns.Get(types.Symbol{Val: name})
		if err != nil {
			t.Errorf("%q not registered", name)
			continue
		}
		fn, ok := v.(types.Func)
		if !ok {
			t.Errorf("%q is %T, expected Func", name, v)
			continue
		}
		if fn.Doc == "" || fn.Arglist == "" {
			t.Errorf("%q missing doc fields: Doc=%q Arglist=%q", name, fn.Doc, fn.Arglist)
		}
		if fn.Meta != nil {
			t.Errorf("%q meta should stay nil, got %v", name, fn.Meta)
		}
	}
}
