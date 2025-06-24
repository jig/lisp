package example

import (
	"context"
	"fmt"
	"log"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/types"
)

func ExampleEVAL() {
	newEnv := env.NewEnv()

	// Load required lisp libraries
	for _, library := range []struct {
		name string
		load func(newEnv types.EnvType) error
	}{
		{"core mal", lisp.LoadNSCore},
		{"core mal with input", lisp.LoadNSCoreInput},
		{"command line args", lisp.LoadNSCoreCmdLineArgs},
		{"concurrent", lisp.LoadNSConcurrent},
		{"core mal extended", nscoreextended.Load},
	} {
		if err := library.load(newEnv); err != nil {
			log.Fatalf("Library Load Error %q: %v", library.name, err)
		}
	}

	// parse (READ) lisp code
	ast, err := lisp.READ(`(+ 2 2)`, nil, newEnv)
	if err != nil {
		log.Fatalf("READ error: %v", err)
	}

	// eval AST
	result, err := lisp.EVAL(context.TODO(), ast, newEnv)
	if err != nil {
		log.Fatalf("EVAL error: %v", err)
	}

	// use result
	if result.(int) != 4 {
		log.Fatalf("Result check error: %v", err)
	}

	// optionally print resulting AST
	fmt.Println(lisp.PRINT(result))
	// Output: 4
}
