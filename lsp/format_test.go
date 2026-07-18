package lsp

import (
	"fmt"
	"strings"
	"testing"
)

// verbTokens extracts the semantic tokens produced for format directives
// (keyword-typed tokens that are not call heads or special forms), as
// "line:start-end" strings.
func verbTokens(t *testing.T, src string) []string {
	t.Helper()
	a := analyseDocument("t.lisp", src)
	idx := newDocIndex(src)
	var out []string
	for _, s := range a.semanticTokens(src) {
		if s.typ != tokKeyword {
			continue
		}
		off := idx.offset(s.rng.Start)
		if off < 0 || src[off] != '%' {
			continue
		}
		out = append(out, tokenSpan(s))
	}
	return out
}

func tokenSpan(s semTok) string {
	return fmt.Sprintf("%d:%d-%d", s.rng.Start.Line, s.rng.Start.Character, s.rng.End.Character)
}

func TestFormatVerbTokens(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		// (format "%s and %03d" a b) — %s at col 9, %03d at col 16
		{`(format "%s and %03d" a b)`, []string{"0:9-11", "0:16-20"}},
		// %% is highlighted too
		{`(format "100%%" )`, []string{"0:12-14"}},
		// truecolor-ish mix of flags, width, precision, star
		{`(format "%-8.2f|%*d" a b c)`, []string{"0:9-15", "0:16-19"}},
		// multi-line string: the verb sits on the second line
		{"(format \"a\n%s\" x)", []string{"1:0-2"}},
		// non-literal format argument: nothing to highlight
		{`(format fmt-var a)`, nil},
		// alternate ¬…¬ string literal
		{`(format ¬%s¬ a)`, []string{"0:10-12"}},
		// printf gets the same treatment
		{`(printf "%s\n" a)`, []string{"0:9-11"}},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got := verbTokens(t, tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("token %d: got %s, want %s", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestFormatDiagnostics(t *testing.T) {
	cases := []struct {
		src     string
		wantMsg []string // substrings, one per expected diagnostic
	}{
		{`(format "%s %d" a b)`, nil},
		{`(format "%s" a b)`, []string{"consumes 1 argument(s), but 2 given"}},
		{`(format "%s %d" a)`, []string{"consumes 2 argument(s), but 1 given"}},
		{`(format "100%%")`, nil},         // %% consumes nothing
		{`(format "%*d" a b)`, nil},       // * consumes an extra argument
		{`(format "%[1]d %[1]d" a)`, nil}, // indexed: count check skipped
		{`(format "abc")`, nil},           // no verbs, no args
		{`(format "%")`, []string{"invalid format directive"}},
		{`(format "% !" a)`, []string{"invalid format directive", "consumes 0 argument(s), but 1 given"}},
		{`(format fmt-var a b)`, nil},      // dynamic format string: ignored
		{`(quote (format "%s" a b))`, nil}, // quoted data: ignored
		{`(defn f [x] (format "%s %s" x))`, []string{"consumes 2 argument(s), but 1 given"}}, // nested in a body
		{"(format \"a\n%d %d\" x)", []string{"consumes 2 argument(s), but 1 given"}},         // multi-line literal
		{`(printf "%s %d" a)`, []string{"consumes 2 argument(s), but 1 given"}},              // printf too
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			a := analyseDocument("t.lisp", tc.src)
			diags := formatDiagnostics(a, tc.src)
			if len(diags) != len(tc.wantMsg) {
				t.Fatalf("got %d diagnostics (%v), want %d", len(diags), diags, len(tc.wantMsg))
			}
			for i, want := range tc.wantMsg {
				if !strings.Contains(diags[i].Message, want) {
					t.Errorf("diagnostic %d = %q, want substring %q", i, diags[i].Message, want)
				}
				if diags[i].Severity != severityWarning {
					t.Errorf("diagnostic %d severity = %d, want warning", i, diags[i].Severity)
				}
			}
		})
	}
}
