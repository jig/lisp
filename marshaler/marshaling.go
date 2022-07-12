package marshaler

import (
	"github.com/jig/lisp/types"
)

type HashMap interface {
	MarshalHashMap() (types.MalType, error)
}

type FactoryJSON interface {
	FromJSON(b []byte) (interface{}, error)
}

type FactoryHashMap interface {
	FromHashMap(data types.MalType) (interface{}, error)
}
