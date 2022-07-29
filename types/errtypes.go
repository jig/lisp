package types

import "fmt"

// type GoError struct {
// 	Obj   MalType
// 	Cause error
// }

// func (terr GoError) Error() string {
// 	switch obj := terr.Obj.(type) {
// 	case string:
// 		return obj
// 	case int:
// 		return fmt.Sprintf("%d", obj)
// 	// case error:
// 	// 	return obj.Error()
// 	default:
// 		return fmt.Sprintf("%v", obj)
// 	}
// }

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

// func NewMalError(err error, cursor *Position) error {
// 	switch err := err.(type) {
// 	case MalError:
// 		// if err.Cursor == nil {
// 		// 	err.Cursor = cursor
// 		// }
// 		return err
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
// 	}
// }

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
