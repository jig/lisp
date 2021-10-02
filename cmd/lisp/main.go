package main

import (
	"log"
	"os"

	"github.com/jig/lisp/command"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/types"
)

func main() {
	repl_env, err := env.NewEnv(nil, nil, nil)
	if err != nil {
		log.Fatalf("Environment Setup Error: %v\n", err)
	}

	for _, library := range []struct {
		name string
		load func(repl_env types.EnvType) error
	}{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs},
		{"core mal extended", nscoreextended.Load},
		{"test", nstest.Load},
	} {
		if err := library.load(repl_env); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	if err := command.Execute(os.Args, repl_env); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
