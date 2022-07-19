package call2

import (
	"errors"
	"fmt"
	"testing"
)

func TestF(t *testing.T) {
	result, error := Call(Div, 2, 6)
	fmt.Println("res", result)
	fmt.Println("err", error)
}

func Div(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("divide by zero")
	}
	return a + b, nil
}
