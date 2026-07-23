package lisp_test

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/lib/system"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/lib/test"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/types"
)

// TestDeftests runs the deftest suite under deftests/ — the deftest
// replica of the .mal step files (see deftests/README.md) — mirroring
// what `lisp --test deftests/` does. Relative paths inside the suite
// (e.g. ./tests/test.txt) resolve against the repository root, which is
// the working directory of this test.
func TestDeftests(t *testing.T) {
	ns := env.NewEnv()
	for _, load := range []func(types.EnvType) error{
		nscore.Load,
		nscore.LoadInput,
		nssystem.Load,
		nscore.LoadNullArgs,
		nsconcurrent.Load,
		nscoreextended.Load,
		nstest.Load,
	} {
		if err := load(ns); err != nil {
			t.Fatalf("library load: %v", err)
		}
	}
	system.Load(ns)
	concurrent.Load(ns)

	files, err := filepath.Glob("deftests/*_test.lisp")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no deftest files found under deftests/")
	}
	sort.Strings(files)

	ctx := context.Background()
	for _, path := range files {
		if _, err := lisp.REPL(ctx, ns, `(load-file "`+path+`")`, types.NewCursorHere(path, -3, 1)); err != nil {
			t.Fatalf("loading %s: %v", path, err)
		}
	}

	reg := test.FromEnv(ns)
	if reg == nil {
		t.Fatal("no test registry in environment")
	}
	results := reg.RunAll(ctx)
	if len(results) == 0 {
		t.Fatal("no tests registered")
	}
	tests, checks := 0, 0
	for _, res := range results {
		tests++
		checks += len(res.Checks)
		if res.OK() {
			continue
		}
		var b strings.Builder
		if res.Err != "" {
			b.WriteString("\n    error: " + res.Err)
		}
		for _, c := range res.Checks {
			if c.OK {
				continue
			}
			b.WriteString("\n    " + c.Module + ":" + strconv.Itoa(c.Line) + ": " + c.Form)
			switch {
			case c.Err != "":
				b.WriteString(": error: " + c.Err)
			case c.Expected != "":
				b.WriteString(": expected " + c.Expected + ", got " + c.Actual)
			default:
				b.WriteString(": failed")
			}
		}
		t.Errorf("FAIL: %s (%s:%d)%s", res.Name, res.Module, res.Line, b.String())
	}
	t.Logf("deftests: %d tests, %d checks", tests, checks)
}
