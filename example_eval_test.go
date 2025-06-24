package lisp_test

import (
	"context"
	"fmt"
	"log"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	. "github.com/jig/lisp/lnotation"
)

func ExampleEVAL() {
	ns := env.NewEnv()
	lisp.LoadNSCore(ns) // to load '+' function

	ast := LS("+", 1, 1)
	result, err := lisp.EVAL(
		context.Background(),
		ast,
		ns,
	)
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	fmt.Printf("result: %d\n", result)

	// Output:
	// result: 2
}
