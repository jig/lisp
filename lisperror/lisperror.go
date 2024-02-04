package lisperror

import (
	"fmt"

	"github.com/jig/lisp/marshaler"
	"github.com/jig/lisp/printer"
	. "github.com/jig/lisp/types"
)

// Errors/Exceptions
type LispError struct {
	err     MalType
	cursor  *Position
	context MalType
}

func (e LispError) Unwrap() error {
	if ee, ok := e.err.(error); ok {
		return ee
	}
	return nil
}

func (e LispError) ErrorValue() MalType {
	switch e.err.(type) {
	case LispError:
		return e.err.(LispError).ErrorValue()
	default:
		return e.err
	}
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
	case LispError:
		// return fmt.Sprintf("\n%s %s: %s", e.cursor, printer.Pr_str(e.context, true), e.err)
		return fmt.Sprintf("\n%s: %s %s", e.cursor, printer.Pr_str(e.context, true), e.err)
	case error:
		return fmt.Sprintf("%s", e.err)
	default:
		return fmt.Sprintf("\n%s %s: %s", e.cursor, printer.Pr_str(e.context, true), e.err)
	}
	// switch e.err.(type) {
	// case error:
	// 	if e.cursor != nil {
	// 		return fmt.Sprintf("%s: %s", e.cursor, e.err)
	// 	}
	// 	return fmt.Sprint(e.err)
	// default:
	// 	// TODO: this should be prt_str
	// 	// panic("internal error: LispError.Error() called on non-error")
	// 	if e.cursor != nil {
	// 		return fmt.Sprintf("%s: %s", e.cursor, e.err)
	// 	}
	// 	return fmt.Sprint(e.err)
	// }
}

// func (e LispError) Stack() []LispError {
// 	stack := []LispError{}
// 	next := e
// 	for {
// 		stack = append(stack, next)

// 		switch nextErr := next.err.(type) {
// 		case LispError:
// 			next = nextErr
// 		default:
// 			// stack = append(stack, LispError{err: nextErr})
// 			return stack
// 		}
// 	}
// }

// func (e LispError) Trace() string {
// 	if len(e.Stack()) < 2 {
// 		return fmt.Sprintf("%s: %s", e.cursor, e.err)
// 	}
// 	res := ""
// 	for _, row := range e.Stack() {
// 		res += fmt.Sprintf("> %s: %s\n->\t%s\n", row.cursor, printer.Pr_str(row.context, true), row.err)
// 	}
// 	return res
// }

func (e LispError) Position() *Position {
	return e.cursor
}

// func (e LispError) LispPrint(_Pr_str func(obj MalType, print_readably bool) string) string {
// 	return "(error " + _Pr_str(e.err, true) + ")"
// }

// func (e LispError) Type() string {
// 	return "error"
// }

// NewGoError is used to create a LispError on errors returned by go functions
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
	pos, err := getPosition(ast)
	if err != nil {
		panic(err)
	}
	return pos
}

func getPosition(ast MalType) (*Position, error) {
	switch value := ast.(type) {
	case List:
		return value.Cursor, nil
	case Symbol:
		return value.Cursor, nil
	case Vector:
		return value.Cursor, nil
	case HashMap:
		return value.Cursor, nil
	case Set:
		return value.Cursor, nil
	case interface{ Position() *Position }:
		return value.Position(), nil
	case *Position:
		return value, nil
	case nil:
		// throw or assert
		return nil, nil
	default:
		return nil, fmt.Errorf("Position(%T)", value)
	}
}

func NewLispError(inErr MalType, contextAST MalType) LispError {
	// log.Print(printer.Pr_str(contextAST, true))
	pos, err := getPosition(contextAST)
	if err != nil {
		return LispError{
			err:     inErr,
			context: contextAST,
		}
	}
	return LispError{
		err:     inErr,
		context: contextAST,
		cursor:  pos,
	}
	// switch err := err.(type) {
	// case LispError:
	// 	err.cursor = GetPosition(ast)
	// 	return err
	// default:
	// 	return LispError{
	// 		err:    err,
	// 		cursor: GetPosition(ast),
	// 	}
	// }
}

func (e LispError) MarshalHashMap() (MalType, error) {
	hm := HashMap{
		Val: map[string]MalType{
			"ʞtype": fmt.Sprintf("%T", e),
		},
	}

	switch ee := e.ErrorValue().(type) {
	case marshaler.HashMap:
		pHm, err := ee.MarshalHashMap()
		if err != nil {
			return nil, err
		}
		hm.Val["ʞerr"] = pHm
	default:
		hm.Val["ʞerr"] = printer.Pr_str(ee, true)
	}

	if e.cursor != nil {
		hm.Val["ʞpos"] = e.cursor.String()
	}

	return hm, nil
}

func (e LispError) LispPrint(Pr_str func(MalType, bool) string) string {
	return "«error " + Pr_str(e.err, true) + "»"
}
