package lisp_test

import (
	"context"
	"fmt"
	"log"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

func ExampleReadEvalWithPreamble() {
	ns := env.NewEnv()
	lisp.LoadNSCore(ns) // to load '+' function

	sourceCode := `;; $ARG 1
(+ 1 $ARG)`
	result, err := lisp.ReadEvalWithPreamble(
		context.Background(),
		ns,
		sourceCode,
		types.NewCursorFile("ExampleReadEvalWithPreamble"),
		nil,
	)
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	fmt.Printf("result: %d\n", result)

	// Output:
	// result: 2
}
