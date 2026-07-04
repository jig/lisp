package system

import (
	"context"
	_ "embed"
	"os"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

func Load(env EnvType) {
	call.Call(env, getenv)
	call.Call(env, setenv)
	call.Call(env, unsetenv)

	call.Doc(env, "getenv", "[name]", "Value of the environment variable name, or nil.")
	call.Doc(env, "setenv", "[name value]", "Sets the environment variable name to value.")
	call.Doc(env, "unsetenv", "[name]", "Removes the environment variable name.")
}

func getenv(ctx context.Context, k string) (MalType, error) {
	if v, ok := os.LookupEnv(k); ok {
		return v, nil
	}
	return nil, nil
}

func setenv(ctx context.Context, k, v string) error {
	return os.Setenv(k, v)
}

func unsetenv(ctx context.Context, k string) error {
	return os.Unsetenv(k)
}
