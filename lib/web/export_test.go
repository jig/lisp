package web

import (
	"net/http"

	. "github.com/jig/lisp/types"
)

// RingHandler exposes the internal ringHandler request→lisp→response
// bridge to the external test package.
func RingHandler(h MalType) http.Handler { return ringHandler(h) }

// VerifyJWT exposes the internal JWT verifier to the external test
// package.
func VerifyJWT(token string, config MalType) (MalType, error) { return webVerifyJWT(token, config) }
