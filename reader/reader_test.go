package reader_test

import (
	"fmt"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"
)

type tests struct {
	name  string
	input string
}

type Example struct {
	N int
	S string
}

func new_example(n int, s string) (Example, error) {
	return Example{
		N: n,
		S: s,
	}, nil
}

func new_error(err types.MalType, cursor ...*types.Position) (lisperror.LispError, error) {
	if len(cursor) == 0 {
		return lisperror.NewLispError(err.(string), nil), nil
	}
	return lisperror.NewLispError(err.(string), cursor[0]), nil
}

func (ex Example) LispPrint(_Pr_str func(obj types.MalType, print_readably bool) string) string {
	return "¡example " + _Pr_str(ex.N, true) + " " + _Pr_str(ex.S, true) + "!"
}

func TestAdHocReaders(t *testing.T) {
	for _, test := range []tests{
		{input: `(hello! "world!")`},
		{input: `¡example 33 "hello"!`},
		{input: `¡error "poum"!`},
		// {input: `¡new-error "poum" nil!`},
	} {
		t.Run(test.name, func(t *testing.T) {
			ns := env.NewEnv()
			if err := nscore.Load(ns); err != nil {
				t.Fatal()
			}
			call.Call(ns, new_example)
			call.Call(ns, new_error, 1, 2)

			ast, err := reader.Read_str(test.input, types.NewCursorFile(t.Name()), nil, ns)
			if err != nil {
				t.Error(err)
			}
			str, err := lisp.PRINT(ast)
			if err != nil {
				t.Error(err)
			}
			fmt.Println(str)
		})
	}
}
