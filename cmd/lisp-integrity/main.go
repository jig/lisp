// Command lisp-integrity runs a jig/lisp script if and only if it (and,
// in cascade, everything it loads as code) matches what is committed in
// its Git repository at HEAD, and attests the run to systemd-journald.
// Integrity is always on and cannot be disabled; there is no REPL, no
// -e and no stdin mode — use the lisp binary for those. See
// INTEGRITY.md for the specification.
package main

import (
	"errors"
	"log"
	"os"

	"github.com/jig/lisp/command"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/cli/nscli"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/lib/git/nsgit"
	"github.com/jig/lisp/lib/integrity/nsintegrity"
	"github.com/jig/lisp/lib/lazy/nslazy"
	"github.com/jig/lisp/lib/log/nslog"
	"github.com/jig/lisp/lib/regexp/nsregexp"
	"github.com/jig/lisp/lib/require/nsrequire"
	"github.com/jig/lisp/lib/sql/nssql"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/lib/term/nsterm"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/lib/web/nsweb"
	"github.com/jig/lisp/types"
)

type library struct {
	name string
	load func(ns types.EnvType) error
}

// libraries mirrors cmd/lisp's list exactly (kept in sync by
// TestLibrariesMatchDocgen) so a verified script sees the same
// environment the lisp binary provides.
func libraries(scriptArgs []string) []library {
	return []library{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs(scriptArgs)},
		{"concurrent", nsconcurrent.Load},
		{"core mal extended", nscoreextended.Load},
		{"system", nssystem.Load},

		// new libraries on jig/lisp v0.3.0
		{"lazy", nslazy.Load},
		{"require", nsrequire.Load("lisp")},
		{"sql", nssql.Load},
		{"cli", nscli.Load},
		{"integrity", nsintegrity.Load},
		{"regexp", nsregexp.Load},
		{"web", nsweb.Load},
		{"git", nsgit.Load},
		{"term", nsterm.Load},
		{"test", nstest.Load},

		// new libraries on jig/lisp v0.6.0
		{"log", nslog.Load},
	}
}

func main() {
	ns := env.NewEnv()

	for _, library := range libraries(command.PreParseIntegrityArgs(os.Args)) {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	if err := command.ExecuteIntegrity(os.Args, ns); err != nil {
		// An integrity failure has already been shown to the user as its
		// own red block; just exit non-zero without a second "Error:" line.
		if errors.Is(err, command.ErrIntegrityReported) {
			os.Exit(1)
		}
		log.Fatalf("Error: %v\n", err)
	}
}
