package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/test"
	"github.com/jig/lisp/types"
)

// discoverTestFiles returns the test files under target, which may be a
// single file or a directory walked recursively. Both the legacy
// *_test.mal suites and *_test.lisp files are picked up.
func discoverTestFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	var files []string
	err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_test.mal") || strings.HasSuffix(info.Name(), "_test.lisp") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// runTests loads every test file under target and then runs the tests
// registered with deftest, reporting on stdout (and optionally as JSON to
// jsonPath). A load error aborts; test failures are reported and produce
// a non-nil error after the whole suite has run.
func runTests(target, jsonPath string, repl_env types.EnvType) error {
	ctx := context.Background()
	files, err := discoverTestFiles(target)
	if err != nil {
		return err
	}
	for _, path := range files {
		name := filepath.Base(path)
		// Legacy *_test.mal suites read *test-params*; keep providing it.
		testParams := fmt.Sprintf(`(def *test-params* {:test-file %q :test-absolute-path %q})`, name, path)
		if _, err := lisp.REPL(ctx, repl_env, testParams, types.NewCursorFile(name)); err != nil {
			return err
		}
		if abs, err := filepath.Abs(path); err == nil {
			registerModule(path, abs)
		}
		if _, err := lisp.REPL(ctx, repl_env, `(load-file "`+path+`")`, types.NewCursorHere(path, -3, 1)); err != nil {
			return err
		}
	}

	reg := test.FromEnv(repl_env)
	var results []*test.Test
	if reg != nil {
		results = reg.RunAll(ctx)
	}

	failedTests := 0
	checks, failedChecks := 0, 0
	for _, t := range results {
		checks += len(t.Checks)
		for _, c := range t.Checks {
			if !c.OK {
				failedChecks++
			}
		}
		if t.OK() {
			continue
		}
		failedTests++
		fmt.Printf("--- FAIL: %s (%s)\n", t.Name, formatPos(t.Module, t.Line))
		if t.Err != "" {
			fmt.Printf("    error: %s\n", indent(t.Err))
		}
		for _, c := range t.Checks {
			if c.OK {
				continue
			}
			fmt.Printf("    %s: %s", formatPos(c.Module, c.Line), c.Form)
			switch {
			case c.Err != "":
				fmt.Printf(": error: %s", indent(c.Err))
			case c.Expected != "":
				fmt.Printf(": expected %s, got %s", c.Expected, c.Actual)
			default:
				fmt.Printf(": failed")
			}
			if c.Message != "" {
				fmt.Printf(" — %s", c.Message)
			}
			fmt.Println()
		}
	}

	if jsonPath != "" {
		if err := writeTestReport(jsonPath, target, results); err != nil {
			return err
		}
	}

	if failedTests > 0 {
		fmt.Printf("FAIL: %d of %d tests failed (%d of %d checks)\n", failedTests, len(results), failedChecks, checks)
		return fmt.Errorf("%d test(s) failed", failedTests)
	}
	fmt.Printf("PASS: %d tests, %d checks\n", len(results), checks)
	return nil
}

func formatPos(module string, line int) string {
	if module == "" {
		return "?"
	}
	if line == 0 {
		return module
	}
	return fmt.Sprintf("%s:%d", module, line)
}

// indent keeps multi-line error messages aligned under their header.
func indent(s string) string {
	return strings.ReplaceAll(s, "\n", "\n        ")
}

// report structures mirror the runner output for tooling (the VS Code
// extension reads this file after a run).
type checkReport struct {
	Form     string `json:"form"`
	OK       bool   `json:"ok"`
	Module   string `json:"module,omitempty"`
	Line     int    `json:"line,omitempty"`
	Error    string `json:"error,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message,omitempty"`
}

type testReport struct {
	Name   string        `json:"name"`
	OK     bool          `json:"ok"`
	Module string        `json:"module,omitempty"`
	Line   int           `json:"line,omitempty"`
	Error  string        `json:"error,omitempty"`
	Checks []checkReport `json:"checks"`
}

type suiteReport struct {
	Target  string       `json:"target"`
	Tests   []testReport `json:"tests"`
	Failed  int          `json:"failed"`
	Checks  int          `json:"checks"`
	FailedC int          `json:"failedChecks"`
}

func writeTestReport(path, target string, results []*test.Test) error {
	rep := suiteReport{Target: target, Tests: []testReport{}}
	for _, t := range results {
		tr := testReport{Name: t.Name, OK: t.OK(), Module: t.Module, Line: t.Line, Error: t.Err, Checks: []checkReport{}}
		for _, c := range t.Checks {
			rep.Checks++
			if !c.OK {
				rep.FailedC++
			}
			tr.Checks = append(tr.Checks, checkReport{
				Form: c.Form, OK: c.OK, Module: c.Module, Line: c.Line,
				Error: c.Err, Expected: c.Expected, Actual: c.Actual, Message: c.Message,
			})
		}
		if !tr.OK {
			rep.Failed++
		}
		rep.Tests = append(rep.Tests, tr)
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
