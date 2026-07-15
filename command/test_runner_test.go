package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/types"
)

func newTestRunnerEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := newTestEnv(t)
	if err := nstest.Load(ns); err != nil {
		t.Fatalf("nstest.Load: %v", err)
	}
	return ns
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunTestsPassAndReport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ok_test.lisp"), `
(deftest sums (is (= 3 (+ 1 2))))
(deftest products (are [x y] (= x y) 4 (* 2 2) 9 (* 3 3)))
`)
	jsonPath := filepath.Join(dir, "report.json")
	if err := runTests(dir, jsonPath, newTestRunnerEnv(t)); err != nil {
		t.Fatalf("runTests: %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var rep suiteReport
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Tests) != 2 || rep.Failed != 0 || rep.Checks != 3 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if rep.Tests[0].Line == 0 || rep.Tests[0].Module == "" {
		t.Fatalf("missing test position: %+v", rep.Tests[0])
	}
}

func TestRunTestsFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad_test.lisp"), `(deftest broken (is (= 1 2)))`)
	jsonPath := filepath.Join(dir, "report.json")
	err := runTests(dir, jsonPath, newTestRunnerEnv(t))
	if err == nil || !strings.Contains(err.Error(), "1 test(s) failed") {
		t.Fatalf("expected failure error, got %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var rep suiteReport
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || rep.Tests[0].Checks[0].Expected != "1" || rep.Tests[0].Checks[0].Actual != "2" {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestRunTestsSingleFileAndLoadError(t *testing.T) {
	dir := t.TempDir()
	single := filepath.Join(dir, "one_test.lisp")
	writeFile(t, single, `(deftest single (is true))`)
	if err := runTests(single, "", newTestRunnerEnv(t)); err != nil {
		t.Fatalf("single file: %v", err)
	}

	broken := filepath.Join(dir, "broken_test.lisp")
	writeFile(t, broken, `(this-symbol-does-not-exist)`)
	if err := runTests(broken, "", newTestRunnerEnv(t)); err == nil {
		t.Fatal("expected a load error")
	}
}
