package types

import (
	"fmt"
	"runtime"
)

// Errors/Exceptions
type MalError struct {
	Obj      MalType
	CausedBy error
	Cursor   *Position
}

func (e MalError) Error() string {
	return fmt.Sprintf("%s", e.Obj)
}

func (e MalError) Position() *Position {
	return e.Cursor
}

func (e MalError) ErrorMessageString() string {
	switch err := e.Obj.(type) {
	case string, runtime.Error, error:
		return fmt.Sprintf("%s: %s", e.Cursor, err)
	default:
		return fmt.Sprintf("%s: %s (%T)", e.Cursor, err, err)
	}
}

// NewGoError is used to create a MalError on errors returned by go functions
func NewGoError(fFullName string, err interface{}) error {
	switch err := err.(type) {
	case interface {
		Unwrap() error
		Error() string
	}:
		return MalError{
			Obj:      fmt.Errorf("%s", fFullName),
			CausedBy: err,
		}
	case error:
		return MalError{
			Obj: fmt.Errorf("%s: %s", fFullName, err),
		}
	case string:
		// TODO(jig): is only called when type mismatch on arguments on a call handled by caller package
		return MalError{
			Obj: fmt.Errorf("%s: %s", fFullName, err),
		}
	default:
		return MalError{
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
	default:
		panic(fmt.Errorf("GetPosition(%T)", value))
	}
}

func SetPosition(e error, ast MalType) error {
	switch e := e.(type) {
	case MalError:
		e.Cursor = GetPosition(ast)
		return e
	default:
		return e
	}
}

func NewMalError(err error, ast MalType) error {
	switch err := err.(type) {
	case MalError:
	case error:
		return MalError{Obj: err}
	default:
		return MalError{Obj: err}
	}
	return SetPosition(err, ast)
}

// // func PushError(cursor *Position, err error) error {
// 	switch err := err.(type) {
// 	case MalError:
// 		// if err.Cursor == nil {
// 		// 	err.Cursor = cursor
// 		// }
// 		return err
// 	case GoError:
// 		return MalError{
// 			Obj:    err.Obj,
// 			Cursor: cursor,
// 		}
// 	case error:
// 		return MalError{
// 			Obj:    err,
// 			Cursor: cursor,
// 		}
// 	default:
// 		return MalError{
// 			Obj:    err,
// 			Cursor: cursor,
// 		}
// 	case nil:
// 		panic(err)
// 	}
// }
