package main

import (
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

// libraries lists every namespace loaded into the interpreter, in order.
// It is the single source of truth shared by main and the docs-coverage
// test (docs_test.go). The docgen package keeps its own copy of this
// list (it is not in an importable package); TestDocgenLibrariesInSync
// guards the two against drift.
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
		{"web", nsweb.Load},
		{"git", nsgit.Load},
		{"term", nsterm.Load},
		{"test", nstest.Load},
	}
}

func main() {
	ns := env.NewEnv()

	for _, library := range libraries(command.PreParseArgs(os.Args)) {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	if err := command.Execute(os.Args, ns); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
