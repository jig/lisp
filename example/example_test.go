package example

import (
	"fmt"
	"log"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/types"
)

func ExampleEVAL() {
	newEnv, err := env.NewEnv(nil, nil, nil)
	if err != nil {
		log.Fatalf("Environment Setup Error: %v", err)
	}

	// Load required lisp libraries
	for _, library := range []struct {
		name string
		load func(newEnv types.EnvType) error
	}{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs},
		{"core mal extended", nscoreextended.Load},
		{"test", nstest.Load},
	} {
		if err := library.load(newEnv); err != nil {
			log.Fatalf("Library Load Error: %v", err)
		}
	}

	// parse (READ) lisp code
	ast, err := lisp.READ(`(+ 2 2)`, nil)
	if err != nil {
		log.Fatalf("READ error: %v", err)
	}

	// eval AST
	result, err := lisp.EVAL(ast, newEnv, nil)
	if err != nil {
		log.Fatalf("EVAL error: %v", err)
	}

	// use result
	if result.(int) != 4 {
		log.Fatalf("Result check error: %v", err)
	}

	// optionally print resulting AST
	resultString, err := lisp.PRINT(result)
	if err != nil {
		log.Fatalf("PRINT error: %v", err)
	}
	fmt.Println(resultString)
	// Output: 4
}
