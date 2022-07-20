package call2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

func TestOK(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	Call(ns, divExample)

	f, err := ns.Get(types.Symbol{Val: "divexample"})
	if err != nil {
		t.Fatal("test failed")
	}
	fcall, ok := f.(externalCall)
	if !ok {
		t.Fatal("test failed")
	}
	result, err := fcall(context.Background(), 2, 6)
	if result.(int) != 8 || err != nil {
		t.Fatal("test failed")
	}
}

func TestNoOKResult(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	Call(ns, divExample)

	f, err := ns.Get(types.Symbol{Val: "divexample"})
	if err != nil {
		t.Fatal("test failed")
	}
	if _, err = f.(externalCall)(context.Background(), 2, 0); err.Error() != "divide by zero" {
		t.Fatal("test failed")
	}
}

func TestNoOKArguments(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	Call(ns, divExample)

	defer func() {
		rerr := recover()
		if rerr.(string) != "wrong number of arguments (3 instead of 2)" {
			t.Fatal("test failed")
		}
	}()
	f, _ := ns.Get(types.Symbol{Val: "divexample"})
	f.(externalCall)(context.Background(), 2, 3, 4)
}

func TestOKWithContext(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	Call(ns, sleepExample)

	f, err := ns.Get(types.Symbol{Val: "sleepexample"})
	if err != nil {
		t.Fatal("test failed")
	}
	if _, err = f.(externalCall)(context.Background(), 10); err != nil {
		t.Fatal("test failed")
	}
}

func TestOKWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ns, _ := env.NewEnv(nil, nil, nil)
	Call(ns, sleepExample)

	f, err := ns.Get(types.Symbol{Val: "sleepexample"})
	if err != nil {
		t.Fatal("test failed")
	}
	if _, err := f.(externalCall)(ctx, 10); err != nil {
		t.Fatal("test failed")
	}
}

func TestPackageRegister(t *testing.T) {
	ns, _ := env.NewEnv(nil, nil, nil)
	Call(ns, sleepExample)
	Call(ns, divExample)
	Call(ns, name_with_hyphens)
	Call(ns, name_With_Caps)

	hm, err := ns.Get(types.Symbol{Val: "_PACKAGES_"})
	if err != nil {
		t.Fatal(err)
	}
	set := hm.(types.HashMap).Val["github.com/jig/lisp/lib/call-helper2"].(types.Set).Val
	if len(set) != 4 {
		t.Fatal("test failed")
	}
	if _, ok := set["divexample"]; !ok {
		t.Fatal("test failed")
	}
	if _, ok := set["sleepexample"]; !ok {
		t.Fatal("test failed")
	}
	if _, ok := set["name-with-hyphens"]; !ok {
		t.Fatal("test failed")
	}
	if _, ok := set["name-with-caps"]; !ok {
		t.Fatal("test failed")
	}
}

func divExample(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("divide by zero")
	}
	return a + b, nil
}

func sleepExample(ctx context.Context, ms int) error {
	select {
	case <-ctx.Done():
		return errors.New("timeout while evaluating expression")
	case <-time.After(time.Millisecond * time.Duration(ms)):
		return nil
	}
}

func name_with_hyphens(ctx context.Context, ms int) error {
	return nil
}

func name_With_Caps(ctx context.Context, ms int) error {
	return nil
}
