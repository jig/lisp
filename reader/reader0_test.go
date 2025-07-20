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
		fmt.Printf("%s\t%s\n", symbol1.Cursor, symbol1.Val)

		symbol2 := list[1].(types.Symbol)
		if symbol2.Val != "a" {
			t.Fatal("Expected 1, got", symbol2.Val)
		}
		fmt.Printf("%s\t%s\n", symbol2.Cursor, symbol2.Val)

		symbol3 := list[2].(types.Symbol)
		if symbol3.Val != "b" {
			t.Fatal("Expected 2, got", symbol3.Val)
		}
		fmt.Printf("%s\t%s\n", symbol3.Cursor, symbol3.Val)
	}
}

func TestBasicMultiline(t *testing.T) {
	ast, err := reader.Read_str("(do\n(+ a b)\n(* c d))\n", types.NewCursorFile(t.Name()), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := ast.(types.List).Val
	if list != nil {
		symbol0 := list[0].(types.Symbol)
		if symbol0.Val != "do" {
			t.Fatal("Expected +, got", symbol0.Val)
		}
		fmt.Printf("%s\t%s\n", symbol0.Cursor, symbol0.Val)

		list1 := list[1].(types.List)
		{
			list := list1.Val
			if list != nil {
				symbol1 := list[0].(types.Symbol)
				if symbol1.Val != "+" {
					t.Fatal("Expected +, got", symbol1.Val)
				}
				fmt.Printf("%s\t%s\n", symbol1.Cursor, symbol1.Val)

				symbol2 := list[1].(types.Symbol)
				if symbol2.Val != "a" {
					t.Fatal("Expected a, got", symbol2.Val)
				}
				fmt.Printf("%s\t%s\n", symbol2.Cursor, symbol2.Val)

				symbol3 := list[2].(types.Symbol)
				if symbol3.Val != "b" {
					t.Fatal("Expected b, got", symbol3.Val)
				}
				fmt.Printf("%s\t%s\n", symbol3.Cursor, symbol3.Val)
			}
		}

		list2 := list[2].(types.List)
		{
			list := list2.Val
			if list != nil {
				symbol1 := list[0].(types.Symbol)
				if symbol1.Val != "*" {
					t.Fatal("Expected *, got", symbol1.Val)
				}
				fmt.Printf("%s\t%s\n", symbol1.Cursor, symbol1.Val)

				symbol2 := list[1].(types.Symbol)
				if symbol2.Val != "c" {
					t.Fatal("Expected c, got", symbol2.Val)
				}
				fmt.Printf("%s\t%s\n", symbol2.Cursor, symbol2.Val)

				symbol3 := list[2].(types.Symbol)
				if symbol3.Val != "d" {
					t.Fatal("Expected d, got", symbol3.Val)
				}
				fmt.Printf("%s\t%s\n", symbol3.Cursor, symbol3.Val)
			}
		}
	}
}
