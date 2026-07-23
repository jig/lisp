package nssystem

import (
	"context"
	"fmt"
	"sync"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/types"
)

// Load registers the system builtins (slurp/spit, chdir/cwd/mkdtemp/…),
// then defines load-file and load-file-once, which read and evaluate a
// file. load-file uses slurp-source (system) plus read-program and eval
// (core), so core and its input layer must already be loaded.
func Load(env types.EnvType) error {
	system.Load(env)

	if _, err := lisp.REPL(context.Background(), env, system.HeaderLoadFile(), types.NewCursorFile("$nssystem")); err != nil {
		return err
	}

	// load-file-once needs mutable state for its seen-set; a Go closure
	// keeps it available without the concurrent library (atoms).
	var mu sync.Mutex
	seen := map[string]bool{}
	env.Set(types.Symbol{Val: "load-file-once"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		if len(a) != 1 {
			return nil, fmt.Errorf("load-file-once: wrong number of arguments (%d instead of 1)", len(a))
		}
		path, ok := a[0].(string)
		if !ok {
			return nil, fmt.Errorf("load-file-once: file-path must be a string (was of type %T)", a[0])
		}
		mu.Lock()
		already := seen[path]
		seen[path] = true
		mu.Unlock()
		if already {
			return nil, nil
		}
		return lisp.EVAL(ctx, types.NewList(nil, types.Symbol{Val: "load-file"}, path), env)
	}})
	call.Doc(env, "load-file-once", "[file-path]", "Like load-file, but never loads the same path twice.")

	return nil
}
