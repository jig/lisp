package lisp

import (
	. "github.com/jig/lisp/types"
)

func L(symbol string, args ...MalType) List {
	return List{
		Val: append(
			[]MalType{
				Symbol{Val: symbol},
			},
			args...,
		),
	}
}

// l := List{
// 	Val: []MalType{
// 		Symbol{Val: "range"},
// 		0, 4,
// 	},
// }
