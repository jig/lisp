package types

import (
	"errors"
	"fmt"
	"strings"
)

// Errors/Exceptions
type malError struct {
	err      MalType
	causedBy error
	cursor   *Position
}

func (e malError) Error() string {
	var errorStr strings.Builder
	errorStr.WriteString(e.ErrorID())

	if cause := e.Unwrap(); cause != nil {
		errorStr.WriteString("\n--")
		errorStr.WriteString(cause.Error())
	}

	return errorStr.String()
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
	case interface {
		Unwrap() error
		Error() string
	}:
		return SetPosition(malError{err: errors.New("new mal error"), causedBy: err}, ast)
	default:
		return SetPosition(malError{err: err}, ast)
	}
}

func (e malError) MarshalHashMap() (MalType, error) {
	hm := HashMap{
		Val: map[string]MalType{
			"ʞerr": e.ErrorID(),
		},
	}

	if e2 := e.Unwrap(); e2 != nil {
		var cause MalType
		var err error
		if m, ok := e2.(malError); ok {
			cause, err = m.MarshalHashMap()
			if err != nil {
				return nil, err
			}
		} else {
			cause = e2.Error()
		}

		hm.Val["ʞcause"] = cause
	}

	if p := e.Position(); p != nil {
		hm.Val["ʞpos"] = p.String()
	}

	return hm, nil
}

type LispMalErrorFactory struct {
	Type malError
}

func new_malerror() (MalType, error) {
	return LispMalErrorFactory{}, nil
}

func (e LispMalErrorFactory) FromHashMap(_hm MalType) (MalType, error) {
	hm := _hm.(HashMap)
	cause := hm.Val["ʞcause"]

	me := malError{
		err: hm.Val["ʞerr"].(string),
	}

	if cause != nil {
		var hmParent MalType
		var err error
		if hmE, ok := cause.(HashMap); ok {
			hmParent, err = e.FromHashMap(hmE)
			if err != nil {
				return nil, err
			}
		} else {
			// if not hashmap it must be a raw string
			hmParent = errors.New(cause.(string))
		}

		me.causedBy = hmParent.(error)
	}

	// TODO: we need complementary of position().string
	// if p,ok := hm.Val["ʞpos"]; ok {
	// 	me.cursor=p
	// }

	return me, nil
}
