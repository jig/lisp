package env

import (
	"testing"

	"github.com/jig/lisp/types"
)

func TestEnv(t *testing.T) {
	year := types.Symbol{Val: "year"}
	ns := NewEnv()
	ns.Set(year, 1984)
	res, err := ns.Get(year)
	if err != nil {
		t.Fatal(err)
	}
	if res.(int) != 1984 {
		t.Fatal()
	}
}
