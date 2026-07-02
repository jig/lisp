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

// testEnv builds an environment with core, coreextended and the require
// library configured with the given include dirs.
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

func repl(t *testing.T, ns types.EnvType, src string) (string, error) {
	t.Helper()
	out, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile("test"))
	s, _ := out.(string)
	return s, err
}

func mustRepl(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	out, err := repl(t, ns, src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return out
}

func TestRequire_QualifiedByDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "greet.lisp"),
		[]byte("(defn hello [n] (str \"hola \" n))\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	mustRepl(t, ns, `(require "greet")`)
	if out := mustRepl(t, ns, `(greet/hello "món")`); out != `"hola món"` {
		t.Errorf("expected \"hola món\", got %v", out)
	}
	// the unqualified name must NOT leak into the root env
	if _, err := repl(t, ns, `(hello "món")`); err == nil {
		t.Error("expected unqualified hello to be undefined")
	}
}

func TestRequire_InternalCrossReferences(t *testing.T) {
	dir := t.TempDir()
	// cube calls sqr internally, unqualified: module-internal references
	// must keep working after namespacing.
	if err := os.WriteFile(filepath.Join(dir, "geom.lisp"),
		[]byte("(defn sqr [x] (* x x))\n(defn cube [x] (* x (sqr x)))\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	mustRepl(t, ns, `(require "geom")`)
	if out := mustRepl(t, ns, `(geom/cube 3)`); out != "27" {
		t.Errorf("expected 27, got %v", out)
	}
}

func TestRequire_As(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "verylongname.lisp"),
		[]byte("(def answer 42)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	mustRepl(t, ns, `(require "verylongname" :as "v")`)
	if out := mustRepl(t, ns, `v/answer`); out != "42" {
		t.Errorf("expected 42, got %v", out)
	}
}

func TestRequire_Refer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mixed.lisp"),
		[]byte("(defn wanted [] 1)\n(defn unwanted [] 2)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	mustRepl(t, ns, `(require "mixed" :refer ["wanted"])`)
	if out := mustRepl(t, ns, `(wanted)`); out != "1" {
		t.Errorf("expected 1, got %v", out)
	}
	// unwanted is only available qualified
	if _, err := repl(t, ns, `(unwanted)`); err == nil {
		t.Error("expected unqualified unwanted to be undefined")
	}
	if out := mustRepl(t, ns, `(mixed/unwanted)`); out != "2" {
		t.Errorf("expected 2, got %v", out)
	}
	// referring a missing symbol errors
	if _, err := repl(t, ns, `(require "mixed" :refer ["nope"])`); err == nil ||
		!strings.Contains(err.Error(), "not found in module") {
		t.Errorf("expected refer error, got %v", err)
	}
}

func TestRequire_LoadsOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	// The module bumps a root-env counter on every EVALUATION (eval runs
	// in the root env).
	if err := os.WriteFile(filepath.Join(dir, "counted.lisp"),
		[]byte("(def bump (eval '(def counter (+ counter 1))))\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	mustRepl(t, ns, `(def counter 0)`)
	mustRepl(t, ns, `(require "counted")`)
	mustRepl(t, ns, `(require "counted")`)
	// re-require with a new alias must not re-evaluate either…
	mustRepl(t, ns, `(require "counted" :as "c2")`)
	if out := mustRepl(t, ns, `counter`); out != "1" {
		t.Errorf("expected module evaluated exactly once (counter 1), got %v", out)
	}
	// …but the alias must exist
	if out := mustRepl(t, ns, `c2/bump`); out != "1" {
		t.Errorf("expected c2/bump = 1, got %v", out)
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
	mustRepl(t, ns, `(require "dup")`)
	if out := mustRepl(t, ns, `dup/which`); out != "1" {
		t.Errorf("expected module from first include dir (1), got %v", out)
	}
}

func TestRequire_NestedModuleName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "my", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my", "lib", "util.lisp"),
		[]byte("(def util-answer 42)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := testEnv(t, dir)
	mustRepl(t, ns, `(require "my/lib/util")`)
	if out := mustRepl(t, ns, `my/lib/util/util-answer`); out != "42" {
		t.Errorf("expected 42, got %v", out)
	}
	mustRepl(t, ns, `(require "my/lib/util" :as "u")`)
	if out := mustRepl(t, ns, `u/util-answer`); out != "42" {
		t.Errorf("expected 42 via alias, got %v", out)
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
