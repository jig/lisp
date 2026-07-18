//go:build debugger

package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoverageLcov runs a small suite under the coverage collector and
// checks the lcov output: the library file is reported with hit and
// unhit lines, and the test file itself is excluded.
func TestCoverageLcov(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "mylib.lisp")
	if err := os.WriteFile(lib, []byte(`(defn double [x] (* 2 x))

(defn never-called [x]
  (str "unused " x))
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my_test.lisp"), []byte(`(load-file "`+lib+`")
(deftest doubles (is (= 4 (double 2))))
`), 0o644); err != nil {
		t.Fatal(err)
	}

	lcov := filepath.Join(dir, "cov.lcov")
	stop, err := startCoverage(lcov)
	if err != nil {
		t.Fatal(err)
	}
	runErr := runTests(dir, "", newTestRunnerEnv(t))
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("runTests: %v", runErr)
	}

	data, err := os.ReadFile(lcov)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "SF:"+lib) {
		t.Fatalf("lcov misses the library file:\n%s", out)
	}
	if strings.Contains(out, "my_test.lisp") {
		t.Fatalf("lcov must exclude test files:\n%s", out)
	}
	// line 1 (defn double) is executed; line 4 (the body of never-called)
	// is parsed as coverable but never hit.
	if !strings.Contains(out, "DA:1,") || strings.Contains(out, "DA:1,0") {
		t.Fatalf("expected line 1 hit:\n%s", out)
	}
	if !strings.Contains(out, "DA:4,0") {
		t.Fatalf("expected line 4 unhit:\n%s", out)
	}
}
