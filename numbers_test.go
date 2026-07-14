package lisp_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
)

func TestInt(t *testing.T) {
	ast, err := lisp.READ("1000", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(int) != 1000 {
		t.Fatal(`ast.(int) != 1000`)
	}
}

func TestIntUnderscoreSeparator(t *testing.T) {
	ast, err := lisp.READ("100_000", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(int) != 100_000 {
		t.Fatal(`ast.(int) != 100_000`)
	}
}

func TestFloat(t *testing.T) {
	ast, err := lisp.READ("3.1416", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(float32) != 3.1416 {
		t.Fatal(`ast.(float32) != 3.1416`)
	}
	res, err := lisp.EVAL(context.Background(), ast, env.NewEnv())
	if err != nil {
		t.Fatal(err)
	}
	if res.(float32) != 3.1416 {
		t.Fatal(`ast.(float32) != 3.1416`)
	}
}

// Radix-prefixed literals read as unsigned arbitrary-precision data
// numbers (*big.Int) and are self-evaluating.
func TestHexa(t *testing.T) {
	ast, err := lisp.READ("0xCAFE", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(*big.Int).Cmp(big.NewInt(0xCAFE)) != 0 {
		t.Fatal(`ast.(*big.Int) != 0xCAFE`)
	}
	res, err := lisp.EVAL(context.Background(), ast, env.NewEnv())
	if err != nil {
		t.Fatal(err)
	}
	if res.(*big.Int).Cmp(big.NewInt(0xCAFE)) != 0 {
		t.Fatal(`res.(*big.Int) != 0xCAFE`)
	}
}

func TestOctal(t *testing.T) {
	ast, err := lisp.READ("0o7777", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(*big.Int).Cmp(big.NewInt(0o7777)) != 0 {
		t.Fatal(`ast.(*big.Int) != 0o7777`)
	}
	res, err := lisp.EVAL(context.Background(), ast, env.NewEnv())
	if err != nil {
		t.Fatal(err)
	}
	if res.(*big.Int).Cmp(big.NewInt(0o7777)) != 0 {
		t.Fatal(`res.(*big.Int) != 0o7777`)
	}
}

func TestBinary(t *testing.T) {
	ast, err := lisp.READ("0b1100", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(*big.Int).Cmp(big.NewInt(0b1100)) != 0 {
		t.Fatal(`ast.(*big.Int) != 0b1100`)
	}
	res, err := lisp.EVAL(context.Background(), ast, env.NewEnv())
	if err != nil {
		t.Fatal(err)
	}
	if res.(*big.Int).Cmp(big.NewInt(0b1100)) != 0 {
		t.Fatal(`res.(*big.Int) != 0b1100`)
	}
}
