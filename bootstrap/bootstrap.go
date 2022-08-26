package bootstrap

import _ "embed"

//go:embed bootstrap.lisp
var bootstrap string

func Code() string {
	return bootstrap
}
