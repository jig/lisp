package types

import (
	"fmt"
)

// Errors/Exceptions
type malError struct {
	err    MalType
	cursor *Position
}

func (e malError) ErrorValue() MalType {
	return e.err
}

func (e malError) Error() string {
	switch e.err.(type) {
	case error:
		if e.cursor != nil {
			return fmt.Sprintf("%s: %s", e.cursor, e.err)
		}
		return fmt.Sprint(e.err)
	default:
		panic("internal error: malError.Error() called on non-error")
	}
}

func (e malError) Position() *Position {
	return e.cursor
}

// func (e malError) LispPrint(_Pr_str func(obj MalType, print_readably bool) string) string {
// 	return "(error " + _Pr_str(e.err, true) + ")"
// }

// func (e malError) Type() string {
// 	return "error"
// }

// NewGoError is used to create a malError on errors returned by go functions
func NewGoError(fFullName string, err interface{}) error {
	switch err := err.(type) {
	case error:
		return malError{
			err: fmt.Errorf("%s: %w", fFullName, err),
		}
	default:
		return malError{
			err: fmt.Errorf("%s: %w", fFullName, fmt.Errorf("%v", err)),
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

func NewMalError(err MalType, ast MalType) error {
	switch err := err.(type) {
	case malError:
		err.cursor = GetPosition(ast)
		return err
	default:
		return malError{
			err:    err,
			cursor: GetPosition(ast),
		}
	}
}
