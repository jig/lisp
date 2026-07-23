package core_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatalf("load core: %v", err)
	}
	if err := nscore.LoadInput(ns); err != nil {
		t.Fatalf("load core input: %v", err)
	}
	if err := nssystem.Load(ns); err != nil {
		t.Fatalf("load system: %v", err)
	}
	return ns
}

func run(t *testing.T, ns types.EnvType, src string) string {
	t.Helper()
	res, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile("test"))
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	s, ok := res.(string)
	if !ok {
		t.Fatalf("REPL returned %T for %q, want printed string", res, src)
	}
	return s
}

func TestSpit(t *testing.T) {
	ns := newEnv(t)
	path := filepath.Join(t.TempDir(), "out.txt")

	if got := run(t, ns, fmt.Sprintf(`(spit %q "hello")`, path)); got != "nil" {
		t.Errorf("spit returned %s, want nil", got)
	}
	if got := run(t, ns, fmt.Sprintf(`(slurp %q)`, path)); got != `"hello"` {
		t.Errorf("after spit, slurp = %s, want \"hello\"", got)
	}

	// Default mode truncates.
	run(t, ns, fmt.Sprintf(`(spit %q "bye")`, path))
	if got := run(t, ns, fmt.Sprintf(`(slurp %q)`, path)); got != `"bye"` {
		t.Errorf("after overwrite, slurp = %s, want \"bye\"", got)
	}

	// :append true appends.
	run(t, ns, fmt.Sprintf(`(spit %q "!" :append true)`, path))
	if got := run(t, ns, fmt.Sprintf(`(slurp %q)`, path)); got != `"bye!"` {
		t.Errorf("after append, slurp = %s, want \"bye!\"", got)
	}

	// :append false behaves like the default.
	run(t, ns, fmt.Sprintf(`(spit %q "fresh" :append false)`, path))
	if got := run(t, ns, fmt.Sprintf(`(slurp %q)`, path)); got != `"fresh"` {
		t.Errorf("after :append false, slurp = %s, want \"fresh\"", got)
	}

	// :append with a file that does not exist yet creates it.
	path2 := filepath.Join(t.TempDir(), "new.txt")
	run(t, ns, fmt.Sprintf(`(spit %q "first" :append true)`, path2))
	if got := run(t, ns, fmt.Sprintf(`(slurp %q)`, path2)); got != `"first"` {
		t.Errorf("append-to-new-file, slurp = %s, want \"first\"", got)
	}
}

func TestSpitErrors(t *testing.T) {
	ns := newEnv(t)
	path := filepath.Join(t.TempDir(), "out.txt")

	for name, src := range map[string]string{
		"odd-options":        fmt.Sprintf(`(spit %q "x" :append)`, path),
		"unknown-option":     fmt.Sprintf(`(spit %q "x" :nonsense true)`, path),
		"non-boolean-append": fmt.Sprintf(`(spit %q "x" :append 1)`, path),
	} {
		if _, err := lisp.REPL(context.Background(), ns, src, types.NewCursorFile("test")); err == nil {
			t.Errorf("%s: %s should error", name, src)
		}
	}

	// A failed spit must not create the file.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file %s should not exist after failed spits (stat err: %v)", path, err)
	}

	// Unwritable path errors.
	if _, err := lisp.REPL(context.Background(), ns,
		`(spit "/nonexistent-dir/x.txt" "x")`, types.NewCursorFile("test")); err == nil {
		t.Error("spit to an unwritable path should error")
	}
}
