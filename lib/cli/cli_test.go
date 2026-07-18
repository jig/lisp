package cli_test

import (
	"context"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/cli/nscli"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/types"
)

func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	for _, load := range []func(types.EnvType) error{
		nscore.Load,
		nscore.LoadInput,
		nsconcurrent.Load,
		nscoreextended.Load, // reduce, filter, map, keyword, … used by the header
		nscli.Load,          // the namespace under test
	} {
		if err := load(ns); err != nil {
			t.Fatalf("load: %v", err)
		}
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

func TestParseOpts(t *testing.T) {
	ns := newEnv(t)

	// A representative spec set: an option with a parse-fn + validate, a
	// boolean flag, and a help flag. Defined once in the env.
	run(t, ns, `(def specs
		[["-p" "--port PORT" "Port number" :default 80 :parse-fn read-string
		  :validate [(fn [n] (and (< 0 n) (< n 65536))) "must be 0..65535"]]
		 ["-v" "--verbose" "Verbosity"]
		 ["-h" "--help" "Show help"]])`)

	cases := []struct{ name, src, want string }{
		// Map fields are read with get to avoid map key-order flakiness.
		{"long-with-arg", `(get (get (cli-parse-opts ["--port" "8080"] specs) :options) :port)`, "8080"},
		{"default-applied", `(get (get (cli-parse-opts [] specs) :options) :port)`, "80"},
		{"short-boolean-flag", `(get (get (cli-parse-opts ["-v"] specs) :options) :verbose)`, "true"},
		{"positional-args", `(get (cli-parse-opts ["-v" "a" "b"] specs) :arguments)`, `["a" "b"]`},
		{"double-dash-separator", `(get (cli-parse-opts ["--" "--port" "x"] specs) :arguments)`, `["--port" "x"]`},
		{"no-errors-when-ok", `(get (cli-parse-opts ["--port" "80"] specs) :errors)`, "nil"},
		{"unknown-option", `(get (cli-parse-opts ["--nope"] specs) :errors)`, `["Unknown option: --nope"]`},
		{"validate-failure", `(get (cli-parse-opts ["--port" "99999"] specs) :errors)`, `["--port: must be 0..65535"]`},
		{"missing-argument", `(get (cli-parse-opts ["--port"] specs) :errors)`, `["Missing argument for --port"]`},
		// *ARGV* is a List, not a Vector: parse-opts must accept both.
		{"accepts-a-list", `(get (get (cli-parse-opts (list "--port" "22") specs) :options) :port)`, "22"},
		// A failed validation must not leave a bad value in :options
		// (the default stays).
		{"invalid-keeps-default", `(get (get (cli-parse-opts ["--port" "0"] specs) :options) :port)`, "80"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(t, ns, tc.src); got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.src, got, tc.want)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	ns := newEnv(t)
	got := run(t, ns, `(cli-summarize [["-v" "--verbose" "Be loud"]])`)
	want := `"  -v --verbose  Be loud\n"`
	if got != want {
		t.Fatalf("summarize = %q, want %q", got, want)
	}
}
