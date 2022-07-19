package call2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jig/lisp/types"
)

func TestOK(t *testing.T) {
	ns := map[string]types.MalType{}
	Call(ns, divExample, false)
	f, ok := ns["divexample"]
	if !ok {
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
	ns := map[string]types.MalType{}
	Call(ns, divExample, false)
	_, err := ns["divexample"].(externalCall)(context.Background(), 2, 0)
	if err.Error() != "divide by zero" {
		t.Fatal("test failed")
	}
}

func TestNoOKArguments(t *testing.T) {
	ns := map[string]types.MalType{}
	Call(ns, divExample, false)
	defer func() {
		rerr := recover()
		if rerr.(string) != "wrong number of arguments (3 instead of 2)" {
			t.Fatal("test failed")
		}
	}()
	_, _ = ns["divexample"].(externalCall)(context.Background(), 2, 3, 4)
}

func TestOKWithContext(t *testing.T) {
	ns := map[string]types.MalType{}
	Call(ns, sleepExample, true)
	_, err := ns["sleepexample"].(externalCall)(context.Background(), 10)
	if err != nil {
		t.Fatal("test failed")
	}
}

func TestOKWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ns := map[string]types.MalType{}
	Call(ns, sleepExample, true)
	_, err := ns["sleepexample"].(externalCall)(ctx, 10)
	if err != nil {
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
