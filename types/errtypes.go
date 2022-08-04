package types

import (
	"errors"
	"fmt"
)

// Errors/Exceptions
type LispError struct {
	err    MalType
	cursor *Position
}

func NewLispError(value MalType) LispError {
	return LispError{
		err:    value,
		cursor: nil,
	}
}

func (e LispError) Unwrap() error {
	if ee, ok := e.err.(error); ok {
		return errors.Unwrap(ee)
	}
	return nil
}

func (e LispError) ErrorValue() MalType {
	return e.err
}

func (e LispError) Is(target error) bool {
	if target == nil {
		return e.ErrorValue() == nil
	}
	// if the error (err.Err) implements its own Is() function use it
	if ee, ok := e.ErrorValue().(interface{ Is(error) bool }); ok && ee.Is(target) {
		return true
	}

	err2, ok := target.(LispError)
	if ok {
		return e.ErrorValue() == err2.ErrorValue()
	}
	return false
}

func (e LispError) Error() string {
	switch e.err.(type) {
	case error:
		if e.cursor != nil {
			return fmt.Sprintf("%s: %s", e.cursor, e.err)
		}
		return fmt.Sprint(e.err)
	default:
		// TODO: this should be prt_str
		// panic("internal error: malError.Error() called on non-error")
		if e.cursor != nil {
			return fmt.Sprintf("%s: %s", e.cursor, e.err)
		}
		return fmt.Sprint(e.err)
	}
}

func (e LispError) Position() *Position {
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
		return LispError{
			err: fmt.Errorf("%s: %w", fFullName, err),
		}
	default:
		return LispError{
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
	case LispError:
		err.cursor = GetPosition(ast)
		return err
	default:
		return LispError{
			err:    err,
			cursor: GetPosition(ast),
		}
	}
}

func (e LispError) LispPrint(pr_str func(obj MalType, print_readably bool) string) string {
	return "(error " + pr_str(e.err, true) + ")"
}
