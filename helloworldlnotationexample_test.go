package lisp

import (
	"context"
	"fmt"
	"log"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	. "github.com/jig/lisp/lnotation"
	"github.com/jig/lisp/types"
)

func ExampleL() {
	var (
		prn = S("prn")
		str = S("str")
	)
	sampleCode := L(prn, L(str, "hello", " ", "world!"))

	result, err := EVAL(sampleCode, newTestEnv(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
	// Output: "hello world!"
	// <nil>
}

func newTestEnv() types.EnvType {
	newEnv, err := env.NewEnv(nil, nil, nil)
	if err != nil {
		log.Fatalf("Environment Setup Error: %v\n", err)
	}
	for k, v := range core.NS {
		newEnv.Set(types.Symbol{Val: k}, types.Func{Fn: v.(func([]types.MalType, *context.Context) (types.MalType, error))})
	}
	return newEnv
}
