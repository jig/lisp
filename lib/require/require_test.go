package require

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/lib/coreextented"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/types"
)

// testEnv builds an environment with core, coreextended (load-file-once)
// and the require library configured with the given include dirs.
func testEnv(t *testing.T, includeDirs ...string) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	core.Load(ns)
	core.LoadInput(ns)
	concurrent.Load(ns)
	if err := nscoreextended.Load(ns); err != nil {
		t.Fatalf("nscoreextended.Load: %v", err)
	}
	ns.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return lisp.EVAL(ctx, a[0], ns)
	}})
	ctx := context.Background()
	for _, header := range []struct {
		name string
		src  string
	}{
		{"basic", core.HeaderBasic()},
		{"load-file", core.HeaderLoadFile()},
		{"concurrent", concurrent.HeaderConcurrent()},
		{"coreextended", coreextented.HeaderCoreExtended()},
	} {
		if _, err := lisp.REPL(ctx, ns, header.src, types.NewCursorFile("preamble")); err != nil {
			t.Fatalf("header %s: %v", header.name, err)
		}
	}
	if err := LoadWithConfig(Config{IncludeDirs: includeDirs})(ns); err != nil {
		t.Fatalf("require.LoadWithConfig: %v", err)
	}
	return ns
}

func TestRequire_LoadsModuleFromIncludeDir(t *testing.T) {
	dir := t.TempDir()
	mod := filepath.Join(dir, "greet.lisp")
	if err := os.WriteFile(mod, []byte("(defn greet [n] (str \"hola \" n))\n"), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	ns := testEnv(t, dir)
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, ns, `(require "greet")`, types.NewCursorFile("test")); err != nil {
		t.Fatalf("require: %v", err)
	}
	out, err := lisp.REPL(ctx, ns, `(greet "món")`, types.NewCursorFile("test"))
	if err != nil {
		t.Fatalf("greet: %v", err)
	}
	if out != `"hola món"` {
		t.Errorf("expected \"hola món\", got %v", out)
	}
}

func TestRequire_NestedModuleName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "my", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	mod := filepath.Join(dir, "my", "lib", "util.lisp")
	if err := os.WriteFile(mod, []byte("(def util-answer 42)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, ns, `(require "my/lib/util")`, types.NewCursorFile("test")); err != nil {
		t.Fatalf("require: %v", err)
	}
	out, err := lisp.REPL(ctx, ns, `util-answer`, types.NewCursorFile("test"))
	if err != nil || out != "42" {
		t.Errorf("expected 42, got %v (err %v)", out, err)
	}
}

func TestRequire_LoadsOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	// The module increments a counter on every load.
	mod := filepath.Join(dir, "counted.lisp")
	if err := os.WriteFile(mod, []byte("(def counter (+ 1 (eval 'counter)))\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, ns, `(def counter 0)`, types.NewCursorFile("test")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := lisp.REPL(ctx, ns, `(require "counted")`, types.NewCursorFile("test")); err != nil {
			t.Fatalf("require #%d: %v", i+1, err)
		}
	}
	out, err := lisp.REPL(ctx, ns, `counter`, types.NewCursorFile("test"))
	if err != nil {
		t.Fatal(err)
	}
	if out != "1" {
		t.Errorf("expected module loaded exactly once (counter 1), got %v", out)
	}
}

func TestRequire_SearchOrder(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	for dir, val := range map[string]string{first: "1", second: "2"} {
		if err := os.WriteFile(filepath.Join(dir, "dup.lisp"), []byte(fmt.Sprintf("(def which %s)\n", val)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ns := testEnv(t, first, second)
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, ns, `(require "dup")`, types.NewCursorFile("test")); err != nil {
		t.Fatal(err)
	}
	out, err := lisp.REPL(ctx, ns, `which`, types.NewCursorFile("test"))
	if err != nil || out != "1" {
		t.Errorf("expected module from first include dir (1), got %v (err %v)", out, err)
	}
}

func TestResolveRequire_RejectsInvalidNames(t *testing.T) {
	config = Config{IncludeDirs: []string{t.TempDir()}}
	for _, bad := range []string{
		"", " ", "/abs/path", "~/home", "../escape", "a/../b", "./x",
		"a//b", ".hidden", "a/.hidden", "with space", "back\\slash",
		"colon:name", "at@name",
	} {
		if _, err := resolve_require(bad); err == nil {
			t.Errorf("expected error for module name %q", bad)
		}
	}
}

func TestResolveRequire_NotFound(t *testing.T) {
	config = Config{IncludeDirs: []string{t.TempDir()}}
	_, err := resolve_require("nope/missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestParseIncludeArgs(t *testing.T) {
	got, err := parseIncludeArgs([]string{"-i", "a", "--include", "b", "--include=c", "-i=d", "--", "-i", "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d: expected %q, got %q", i, want[i], got[i])
		}
	}
	if _, err := parseIncludeArgs([]string{"-i"}); err == nil {
		t.Error("expected error for dangling -i")
	}
}
