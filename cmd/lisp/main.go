package main

import (
	"log"
	"os"

	"github.com/jig/lisp"
	"github.com/jig/lisp/command"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/assert/nsassert"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/types"
)

func main() {
	ns := env.NewEnv()

	for _, library := range []struct {
		name string
		load func(ns types.EnvType) error
	}{
		{"core mal", lisp.LoadNSCore},
		{"core mal with input", lisp.LoadNSCoreInput},
		{"command line args", lisp.LoadNSCoreCmdLineArgs},
		{"concurrent", lisp.LoadNSConcurrent},
		{"core mal extended", func(ns types.EnvType) error { return nscoreextended.Load(ns, nil) }},
		{"assert", func(ns types.EnvType) error { return nsassert.Load(ns, nil) }},
		{"system", func(ns types.EnvType) error { return nssystem.Load(ns, nil) }},
	} {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	if err := command.Execute(os.Args, ns); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
