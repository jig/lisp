package types

import (
	"fmt"
	"runtime"
)

// Errors/Exceptions
type malError struct {
	err      MalType
	causedBy error
	cursor   *Position
}

func (e malError) Error() string {
	switch e.err.(type) {
	case string, runtime.Error, error:
		if e.cursor != nil {
			return fmt.Sprintf("%s: %s", e.cursor, e.err)
		}
		return fmt.Sprintf("%s", e.err)
	default:
		if e.cursor != nil {
			return fmt.Sprintf("%s: %[1]s (%[1]T)", e.cursor, e.err)
		}
		return fmt.Sprintf("%s", e.err)
	}
}

func (e malError) Unwrap() error {
	return e.causedBy
}

func (err *malError) Is(target error) bool {
	if target == nil {
		return err == nil
	}
	if e, ok := err.err.(interface{ Is(error) bool }); ok && e.Is(target) {
		return true
	}
	if err2, ok := target.(malError); ok {
		return err.ErrorID() == err2.ErrorID()
	}
	return err.ErrorID() == target.Error()
}

func (e malError) ErrorID() string {
	return fmt.Sprintf("%s", e.err)
}

func (e malError) ErrorEncapsuled() MalType {
	return e.err
}

func (e malError) Position() *Position {
	return e.cursor
}

// NewGoError is used to create a malError on errors returned by go functions
func NewGoError(fFullName string, err interface{}) error {
	switch err := err.(type) {
	case interface {
		Unwrap() error
		Error() string
	}:
		return malError{
			err:      fmt.Errorf("%s", fFullName),
			causedBy: err,
		}
	case error:
		return malError{
			err: fmt.Errorf("%s: %s", fFullName, err),
		}
	case string:
		// TODO(jig): is only called when type mismatch on arguments on a call handled by caller package
		return malError{
			err: fmt.Errorf("%s: %s", fFullName, err),
		}
	default:
		return malError{
			err: fmt.Errorf("%s: %s", fFullName, err),
		}
	}
}

func GetPosition(ast MalType) *Position {
	switch value := ast.(type) {
	case List:
		return value.Cursor
	case Symbol:
		return value.Cursor
	case Vector:
		return value.Cursor
	case HashMap:
		return value.Cursor
	case Set:
		return value.Cursor
	case interface{ GetPosition() *Position }:
		return value.GetPosition()
	case *Position:
		return value
	case nil:
		// throw or assert
		return nil
	default:
		panic(fmt.Errorf("GetPosition(%T)", value))
	}
}

func SetPosition(e error, ast MalType) error {
	switch e := e.(type) {
	case malError:
		e.cursor = GetPosition(ast)
		return e
	case nil:
		// used by throw and assert
		return e
	default:
		return e
	}
}

func NewMalError(err MalType, ast MalType) error {
	switch err := err.(type) {
	case malError:
		return SetPosition(err, ast)
	case error:
		return SetPosition(malError{err: err}, ast)
	default:
		return SetPosition(malError{err: err}, ast)
	}
}
