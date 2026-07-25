package docgen

import (
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

// library is one namespace to document: its display name and its loader.
type library struct {
	name string
	load func(ns types.EnvType) error
}

// standardLibraries lists the namespaces documented in LANGUAGE.md, in
// load order. It deliberately mirrors cmd/lisp's own libraries() list so
// the reference describes exactly what the binary ships; the two lists
// live in different packages (cmd/lisp is package main and cannot be
// imported), and TestDocgenLibrariesInSync (in cmd/lisp) fails if they
// drift apart.
func standardLibraries() []library {
	return []library{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs(nil)},
		{"concurrent", nsconcurrent.Load},
		{"core mal extended", nscoreextended.Load},
		{"system", nssystem.Load},
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
		{"log", nslog.Load},
	}
}

// StandardSymbols loads every documented library and returns the sorted
// names bound in the resulting environment. It exists so a test in the
// cmd/lisp package can assert docgen documents exactly the environment
// the binary builds.
func StandardSymbols() ([]string, error) {
	ns := env.NewEnv()
	for _, lib := range standardLibraries() {
		if err := lib.load(ns); err != nil {
			return nil, err
		}
	}
	return ns.(*env.Env).LocalSymbols(), nil
}
