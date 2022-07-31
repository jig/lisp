package main

import (
	"log"
	"os"

	"github.com/jig/lisp"
	"github.com/jig/lisp/command"
	"github.com/jig/lisp/debugger"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/types"
)

func main() {
	ns := env.NewEnv()

	for _, library := range []struct {
		name string
		load func(ns types.EnvType) error
	}{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs},
		{"core mal extended", nscoreextended.Load},
		{"test", nstest.Load},
		{"concurrent", nsconcurrent.Load},
	} {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <file>\n", os.Args[0])
	}

	deb := debugger.Engine(os.Args[1], ns)
	defer deb.Shutdown()
	lisp.Stepper = deb.Stepper

	if err := command.Execute(os.Args, ns); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
