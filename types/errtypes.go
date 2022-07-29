package types

import (
	"errors"
	"fmt"
	"runtime"
)

// Errors/Exceptions
type malError struct {
	Obj      MalType
	CausedBy error
	Cursor   *Position
}

func (e malError) Error() string {
	return fmt.Sprintf("%s", e.Obj)
}

func (e malError) Unwrap() error {
	return e.CausedBy
}

func (e malError) ErrorEncapsuled() MalType {
	return e.Obj
}

func (e malError) Position() *Position {
	return e.Cursor
}

func (e malError) ErrorMessageString() string {
	switch err := e.Obj.(type) {
	case string, runtime.Error, error:
		return fmt.Sprintf("%s: %s", e.Cursor, err)
	default:
		return fmt.Sprintf("%s: %s (%T)", e.Cursor, err, err)
	}
}

func ErrorMessageStack(err error) string {
	parentErr := errors.Unwrap(err)

	switch errTyped := err.(type) {
	case interface{ ErrorMessageString() string }:
		if parentErr != nil {
			return errTyped.ErrorMessageString() + "\n" + ErrorMessageStack(parentErr)
		}
		return errTyped.ErrorMessageString()
	default:
		if parentErr != nil {
			return err.Error() + "\n" + ErrorMessageStack(errors.Unwrap(err))
		}
		return errTyped.Error()
	}
}

// NewGoError is used to create a malError on errors returned by go functions
func NewGoError(fFullName string, err interface{}) error {
	switch err := err.(type) {
	case interface {
		Unwrap() error
		Error() string
	}:
		return malError{
			Obj:      fmt.Errorf("%s", fFullName),
			CausedBy: err,
		}
	case error:
		return malError{
			Obj: fmt.Errorf("%s: %s", fFullName, err),
		}
	case string:
		// TODO(jig): is only called when type mismatch on arguments on a call handled by caller package
		return malError{
			Obj: fmt.Errorf("%s: %s", fFullName, err),
		}
	default:
		return malError{
			Obj: fmt.Errorf("%s: %s", fFullName, err),
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
		e.Cursor = GetPosition(ast)
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
		return SetPosition(malError{Obj: err}, ast)
	default:
		return SetPosition(malError{Obj: err}, ast)
	}
}
