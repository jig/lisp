package lnotation

import (
	. "github.com/jig/lisp/types"
)

// S converts argument string to a lisp symbol
func S(arg string) Symbol {
	return Symbol{Val: arg}
}

// L returns a lisp list of its arguments
func L(args ...MalType) List {
	return List{Val: args}
}

// LS is a helper to call (f ...) lisp forms
func LS(symbol string, args ...MalType) List {
	return List{
		Val: append(
			[]MalType{
				Symbol{Val: symbol},
			},
			args...,
		),
	}
}

// V returns a lisp vector of its arguments
func V(args ...MalType) Vector {
	return Vector{Val: args}
}

// M converts Go map to lisp HashMap
func M(arg map[string]MalType) HashMap {
	return HashMap{Val: arg}
}
