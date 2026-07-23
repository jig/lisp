package system

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

func Load(env EnvType) {
	call.Call(env, getenv)
	call.Call(env, setenv)
	call.Call(env, unsetenv)
	call.Call(env, chdir)
	call.Call(env, cwd)
	call.Call(env, mkdtemp, 0, 1)
	call.Call(env, remove_all)

	call.Doc(env, "getenv", "[name]", "Value of the environment variable name, or nil.")
	call.Doc(env, "setenv", "[name value]", "Sets the environment variable name to value.")
	call.Doc(env, "unsetenv", "[name]", "Removes the environment variable name.")
	call.Doc(env, "chdir", "[path]",
		"Changes the process working directory to path. Process-global: affects the working directory of all subsequent operations, including the .state store and git repository detection.")
	call.Doc(env, "cwd", "[]",
		"Returns the process working directory as an absolute path string.")
	call.Doc(env, "mkdtemp", "[& prefix]",
		"Creates a new uniquely-named temporary directory (optionally name-prefixed) and returns its absolute path.")
	call.Doc(env, "remove-all", "[path]",
		"Recursively removes path and everything under it; does not error if path is absent.")
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

// chdir changes the process-global working directory.
func chdir(path string) (MalType, error) {
	if err := os.Chdir(path); err != nil {
		return nil, err
	}
	return nil, nil
}

// cwd returns the absolute process working directory.
func cwd() (MalType, error) {
	return os.Getwd()
}

// mkdtemp creates a new temporary directory and returns its path.
func mkdtemp(prefix ...MalType) (MalType, error) {
	p := ""
	if len(prefix) == 1 {
		s, ok := prefix[0].(string)
		if !ok {
			return nil, fmt.Errorf("mkdtemp: prefix must be a string, got %T", prefix[0])
		}
		p = s
	}
	return os.MkdirTemp("", p)
}

// remove_all recursively removes path (lisp name: remove-all).
func remove_all(path string) (MalType, error) {
	return nil, os.RemoveAll(path)
}
