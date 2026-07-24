//go:build debugger

package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

// covCollector is an EvalHook that counts evaluated forms per source
// line. It chains to any previously installed hook so --coverage can
// coexist with --debug.
type covCollector struct {
	next runtime.EvalHook
	// hits: module → line → evaluation count. Access is not synchronised:
	// the runner evaluates on one goroutine; forms evaluated inside
	// (future …) run detached and are counted too, racing benignly at
	// worst on a counter — acceptable for coverage. A mutex would sit on
	// the interpreter's hottest path.
	hits map[string]map[int]int
}

func (c *covCollector) OnEval(ctx context.Context, ev runtime.EvalEvent) error {
	if ev.Cursor != nil && ev.Cursor.Module != nil && ev.Cursor.BeginRow > 0 {
		lines, ok := c.hits[*ev.Cursor.Module]
		if !ok {
			lines = map[int]int{}
			c.hits[*ev.Cursor.Module] = lines
		}
		lines[ev.Cursor.BeginRow]++
	}
	if c.next != nil {
		return c.next.OnEval(ctx, ev)
	}
	return nil
}

// startCoverage installs the collecting hook and returns a stop function
// that uninstalls it and writes the lcov report.
func startCoverage(path string) (func() error, error) {
	col := &covCollector{next: runtime.Hook, hits: map[string]map[int]int{}}
	runtime.Hook = col
	return func() error {
		runtime.Hook = col.next
		return writeLcov(path, col.hits)
	}, nil
}

// coverableLines parses a lisp source file and returns the set of lines
// holding at least one form — the denominator of the coverage report.
func coverableLines(path string) (map[int]bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ast, err := reader.Read_program(string(src), types.NewCursorFile(path), nil)
	if err != nil {
		return nil, err
	}
	lines := map[int]bool{}
	var walk func(form types.MalType)
	mark := func(cur *types.Position) {
		if cur != nil && cur.BeginRow > 0 {
			lines[cur.BeginRow] = true
		}
	}
	walk = func(form types.MalType) {
		switch f := form.(type) {
		case types.List:
			mark(f.Cursor)
			for _, c := range f.Val {
				walk(c)
			}
		case types.Vector:
			mark(f.Cursor)
			for _, c := range f.Val {
				walk(c)
			}
		case types.HashMap:
			mark(f.Cursor)
			for _, c := range f.Items {
				walk(c)
			}
		case types.Symbol:
			mark(f.Cursor)
		}
	}
	if do, ok := ast.(types.List); ok && len(do.Val) > 1 {
		for _, c := range do.Val[1:] {
			walk(c)
		}
	}
	return lines, nil
}

// resolveModule maps a Cursor module identifier to an absolute path of an
// existing regular file, or "" when the module does not correspond to a
// coverable source file (REPL, -e, $-prefixed library headers, …).
func resolveModule(module string) string {
	path := module
	if p, ok := runtime.Modules.Lookup(module); ok {
		path = p
	}
	if strings.HasPrefix(filepath.Base(path), "$") {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return ""
	}
	return abs
}

// isTestFile reports whether path is a test source, excluded from the
// coverage report the way Go excludes _test.go files.
func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.lisp") || strings.HasSuffix(path, "_test.mal")
}

// writeLcov merges hit counts into per-file line records and writes an
// lcov tracefile. Lines are the union of the statically coverable lines
// and the lines actually hit (macro-expanded code may execute lines the
// static walk cannot attribute).
func writeLcov(path string, hits map[string]map[int]int) error {
	// Merge modules resolving to the same file (relative vs absolute).
	byFile := map[string]map[int]int{}
	for module, lines := range hits {
		abs := resolveModule(module)
		if abs == "" || isTestFile(abs) {
			continue
		}
		m, ok := byFile[abs]
		if !ok {
			m = map[int]int{}
			byFile[abs] = m
		}
		for line, n := range lines {
			m[line] += n
		}
	}

	var b strings.Builder
	b.WriteString("TN:\n")
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, file := range files {
		lineHits := byFile[file]
		all := map[int]int{}
		if coverable, err := coverableLines(file); err == nil {
			for line := range coverable {
				all[line] = 0
			}
		}
		for line, n := range lineHits {
			all[line] += n
		}
		lines := make([]int, 0, len(all))
		for line := range all {
			lines = append(lines, line)
		}
		sort.Ints(lines)
		fmt.Fprintf(&b, "SF:%s\n", file)
		covered := 0
		for _, line := range lines {
			fmt.Fprintf(&b, "DA:%d,%d\n", line, all[line])
			if all[line] > 0 {
				covered++
			}
		}
		fmt.Fprintf(&b, "LF:%d\nLH:%d\nend_of_record\n", len(lines), covered)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
