package types

import (
	"errors"
	"testing"
)

func TestMalErrorMarshal(t *testing.T) {
	pe := malError{
		err: errors.New("Parent example error"),
	}
	e := malError{
		err:      errors.New("Child example error"),
		causedBy: pe,
	}
	h, err := e.MarshalHashMap()
	if err != nil {
		t.Fatal(err)
	}
	newE, err := LispMalErrorFactory{Type: malError{}}.FromHashMap(h)
	if err != nil {
		t.Fatal(err)
	}
	if newE.(malError).ErrorID() != "Child example error" {
		t.Fatal("test failed")
	}
	if newE.(malError).Unwrap().(malError).ErrorID() != "Parent example error" {
		t.Fatal("test failed")
	}
}
