package reader_test

import (
	"fmt"
	"testing"

	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"
)

func TestBasic(t *testing.T) {
	ast, err := reader.Read_str(`(+ a b)`, types.NewCursorFile(t.Name()), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := ast.(types.List).Val
	if list != nil {
		symbol1 := list[0].(types.Symbol)
		if symbol1.Val != "+" {
			t.Fatal("Expected +, got", symbol1.Val)
		}
		fmt.Printf("pos: %+v\n", symbol1.Cursor)

		symbol2 := list[1].(types.Symbol)
		if symbol2.Val != "a" {
			t.Fatal("Expected 1, got", symbol2.Val)
		}
		fmt.Printf("pos: %+v\n", symbol2.Cursor)

		symbol3 := list[2].(types.Symbol)
		if symbol3.Val != "b" {
			t.Fatal("Expected 2, got", symbol3.Val)
		}
		fmt.Printf("pos: %+v\n", symbol3.Cursor)
	}
}
