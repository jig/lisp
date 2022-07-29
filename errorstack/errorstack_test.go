package errorstack

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorStack(t *testing.T) {
	err1 := fmt.Errorf("Error: %w", errors.New("core dump"))
	err2 := fmt.Errorf("Error: %w", err1)
	err3 := fmt.Errorf("Error: %w", err2)

	fmt.Println(err3)
	fmt.Println(Dump(err3))
}
