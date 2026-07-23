package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolve follows symlinks so comparisons hold on macOS, where
// t.TempDir()/os.MkdirTemp live under /var → /private/var.
func resolve(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestChdirCwd(t *testing.T) {
	dir := t.TempDir() // registers its own cleanup first…

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// …so this restore, registered after, runs BEFORE dir is removed —
	// leaving cwd valid and not polluting other tests in the package.
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if _, err := chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got, err := cwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	if resolve(t, got.(string)) != resolve(t, dir) {
		t.Fatalf("cwd = %q, want %q", got, dir)
	}

	// chdir to a missing directory errors and leaves cwd unchanged.
	if _, err := chdir(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("chdir to a missing directory should error")
	}
	if now, _ := os.Getwd(); resolve(t, now) != resolve(t, dir) {
		t.Fatalf("failed chdir changed cwd to %q", now)
	}
}

func TestMkdtemp(t *testing.T) {
	a, err := mkdtemp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(a.(string)) })
	if st, err := os.Stat(a.(string)); err != nil || !st.IsDir() {
		t.Fatalf("mkdtemp did not create a directory: %v", err)
	}

	b, err := mkdtemp("jig-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(b.(string)) })
	if a.(string) == b.(string) {
		t.Fatal("two mkdtemp calls returned the same path")
	}
	if !strings.HasPrefix(filepath.Base(b.(string)), "jig-") {
		t.Fatalf("prefix not applied: %s", b)
	}

	if _, err := mkdtemp(42); err == nil {
		t.Fatal("a non-string prefix should error")
	}
}

func TestRemoveAll(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "a")
	if _, err := remove_all(target); err != nil {
		t.Fatalf("remove-all: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("remove-all left %s behind", target)
	}

	// an absent path is not an error
	if _, err := remove_all(filepath.Join(dir, "missing")); err != nil {
		t.Fatalf("remove-all on an absent path errored: %v", err)
	}
}
