package system

import (
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

// TestSystemDocs asserts the system builtins carry documentation
// (Doc/Arglist fields) after Load, with nil meta (kanaka/mal
// compatibility).
func TestSystemDocs(t *testing.T) {
	ns := env.NewEnv()
	Load(ns)
	for _, name := range []string{"getenv", "setenv", "unsetenv"} {
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
			t.Errorf("%q missing doc fields", name)
		}
		if fn.Meta != nil {
			t.Errorf("%q meta should stay nil, got %v", name, fn.Meta)
		}
	}
}
