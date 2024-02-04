package lisp_test

import (
	"testing"

	"github.com/jig/lisp"
)

func TestBase(t *testing.T) {
	ast, err := lisp.READ(`"hello"`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ast.(string) != `hello` {
		t.Fatal(ast)
	}
}

func TestErrorParsingUnterminatedString(t *testing.T) {
	ast, err := lisp.READ(`"hello`, nil, nil)
	if err == nil {
		t.Fatalf("must throw error but returns %q", ast)
	}
	// switch err := err.(type) {
	// case lisperror.LispError:
	// 	if fmt.Sprintf("%s", err.ErrorValue()) != `invalid token "hello` {
	// 		t.Fatalf("%s-%s", `invalid token "hello`, err.ErrorValue())
	// 	}
	// case error:
	expectedError := "\n§L1,C1: §L1,C1 invalid token \"hello"
	if err.Error() != expectedError {
		t.Fatalf("%q != %q", err.Error(), expectedError)
	}
	// default:
	// 	t.Fatal(err)
	// }
	// if err.ErrorValue() != `invalid token "hello` {
	// 	t.Fatal(err)
	// }
}

func TestErrorParsingUnterminatedHexa(t *testing.T) {
	ast, err := lisp.READ(`0xXYZ`, nil, nil)
	if err == nil {
		t.Fatalf("must throw error but returns %q", ast)
	}
	if err.Error() != "\n§L1,C1: §L1,C1 invalid token 0x" {
		t.Fatal(err.Error())
	}
}
