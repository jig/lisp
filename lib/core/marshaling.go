package core

import (
	"github.com/jig/lisp/types"
)

type HashMapMarshaler interface {
	MarshalHashMap() (types.MalType, error)
}

type FactoryUnmarshalJson interface {
	// not to confuse linter with UnmarshalJSON for custom JSON unmarshaler
	UnmarshalJson(b []byte) (interface{}, error)
}

type FactoryUnmarshalHashMap interface {
	UnmarshalHashMap(data types.MalType) (interface{}, error)
}
