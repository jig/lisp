package format

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSource(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "reindent to two spaces",
			in:   "(defn f\n    [n]\n        (+ n 1))\n",
			want: "(defn f\n  [n]\n  (+ n 1))\n",
		},
		{
			name: "single line untouched",
			in:   "(+ 1 2 3)\n",
			want: "(+ 1 2 3)\n",
		},
		{
			name: "shebang preserved verbatim",
			in:   "#!/usr/bin/env lisp\n(println   1)\n",
			want: "#!/usr/bin/env lisp\n(println 1)\n",
		},
		{
			name: "vector aligns to first element",
			in:   "(let [a 1\n   b 2]\n  a)\n",
			want: "(let [a 1\n      b 2]\n  a)\n",
		},
		{
			name: "collapse inter-token whitespace",
			in:   "(+   1    2)\n",
			want: "(+ 1 2)\n",
		},
		{
			name: "collapse blank lines to one",
			in:   "(a)\n\n\n\n(b)\n",
			want: "(a)\n\n(b)\n",
		},
		{
			name: "trailing comment kept on line",
			in:   "(def x 1) ; the x\n",
			want: "(def x 1) ; the x\n",
		},
		{
			name: "standalone comment indented with body",
			in:   "(do\n; a note\n(f))\n",
			want: "(do\n  ; a note\n  (f))\n",
		},
		{
			name: "comment before closer drops closer to its own line",
			in:   "(list\n  1 ; one\n)\n",
			want: "(list\n  1 ; one\n)\n",
		},
		{
			name: "close delimiters hug the last element",
			in:   "(a (b (c)\n)\n)\n",
			want: "(a (b (c)))\n",
		},
		{
			name: "reader macros hug their form",
			in:   "( quote  x )\n'foo\n`(a ~b ~@c)\n@state\n^{:m 1} y\n",
			want: "(quote x)\n'foo\n`(a ~b ~@c)\n@state\n^{:m 1} y\n",
		},
		{
			name: "raw string preserved verbatim",
			in:   "(def d\n¬line one\n  line two¬\n1)\n",
			want: "(def d\n  ¬line one\n  line two¬\n  1)\n",
		},
		{
			name: "trailing newline added",
			in:   "(a)",
			want: "(a)\n",
		},
		{
			name: "leading blank lines removed",
			in:   "\n\n(a)\n",
			want: "(a)\n",
		},
		{
			name: "empty list stays compact",
			in:   "(  )\n",
			want: "()\n",
		},
		{
			name: "set literal aligns to first element",
			in:   "#{1\n2}\n",
			want: "#{1\n  2}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Source([]byte(tc.in))
			if err != nil {
				t.Fatalf("Source() error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Source() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
			// idempotency: formatting the output again must be a no-op
			again, err := Source(got)
			if err != nil {
				t.Fatalf("Source() second pass error: %v", err)
			}
			if string(again) != string(got) {
				t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", got, again)
			}
		})
	}
}

func TestSourceErrors(t *testing.T) {
	cases := map[string]string{
		"unbalanced open":  "(a (b)",
		"unbalanced close": "(a))",
		"mismatched":       "(a]",
		"unterminated str": "(a \"oops)",
		"unterminated raw": "(a ¬oops)",
		"dangling prefix":  "(a ')",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Source([]byte(in)); err == nil {
				t.Errorf("expected error for %q, got none", in)
			}
		})
	}
}

// TestExamplesIdempotent formats every example file twice and checks the
// formatter reaches a fixed point on real-world source.
func TestExamplesIdempotent(t *testing.T) {
	files, _ := filepath.Glob("../examples/*.lisp")
	if len(files) == 0 {
		t.Skip("no example files found")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			once, err := Source(src)
			if err != nil {
				t.Fatalf("Source() error: %v", err)
			}
			twice, err := Source(once)
			if err != nil {
				t.Fatalf("Source() second pass error: %v", err)
			}
			if string(once) != string(twice) {
				t.Errorf("%s not idempotent", f)
			}
		})
	}
}
