package env

import (
	"reflect"
	"testing"

	"github.com/jig/lisp/types"
)

func TestRemove(t *testing.T) {
	x := types.Symbol{Val: "x"}
	ns := NewEnv()
	ns.Set(x, 1)

	if err := ns.Remove(x); err != nil {
		t.Fatalf("Remove existing: %v", err)
	}
	if _, err := ns.Get(x); err == nil {
		t.Fatal("Get after Remove should fail")
	}
	if err := ns.Remove(x); err == nil {
		t.Fatal("Remove of a missing symbol should error")
	}
}

func TestOuter(t *testing.T) {
	base := NewEnv()
	if base.(*Env).Outer() != nil {
		t.Fatal("root env Outer() should be nil")
	}
	child := NewSubordinateEnv(base)
	if child.(*Env).Outer() == nil {
		t.Fatal("child Outer() should be the base env")
	}
}

func TestLocalSymbols(t *testing.T) {
	base := NewEnv()
	base.(*Env).Set(types.Symbol{Val: "outer"}, 0)

	child := NewSubordinateEnv(base).(*Env)
	child.Set(types.Symbol{Val: "b"}, 1)
	child.Set(types.Symbol{Val: "a"}, 2)
	child.Set(types.Symbol{Val: "c"}, 3)

	// Only this env's own bindings, sorted, no outer names.
	if got, want := child.LocalSymbols(), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("LocalSymbols = %v, want %v", got, want)
	}
}
