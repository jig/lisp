package call2

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOK(t *testing.T) {
	fDiv := Call(divExample)
	result, err := fDiv(context.TODO(), 2, 6)
	if result.(int) != 8 || err != nil {
		t.Fatal("test failed")
	}
}

func TestNoOKResult(t *testing.T) {
	fDiv := Call(divExample)
	_, err := fDiv(context.TODO(), 2, 0)
	if err.Error() != "divide by zero" {
		t.Fatal("test failed")
	}
}

func TestNoOKArguments(t *testing.T) {
	fDiv := Call(divExample)
	defer func() {
		rerr := recover()
		if rerr.(string) != "wrong number of arguments (3 instead of 2)" {
			t.Fatal("test failed")
		}
	}()
	_, _ = fDiv(context.TODO(), 2, 3, 4)
}

func TestOKWithContext(t *testing.T) {
	fSleep := CallWithContext(sleepExample)
	_, err := fSleep(context.Background(), 10)
	if err != nil {
		t.Fatal("test failed")
	}
}

func TestOKWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fSleep := CallWithContext(sleepExample)
	_, err := fSleep(ctx, 20)
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
