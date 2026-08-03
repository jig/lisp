package system

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/printer"
	lispruntime "github.com/jig/lisp/runtime"
	. "github.com/jig/lisp/types"
)

//go:embed header-load-file.lisp
var headerLoadFile string

// HeaderLoadFile returns the lisp source defining load-file; nssystem
// evaluates it after the system builtins (slurp-source) are registered.
func HeaderLoadFile() string { return headerLoadFile }

func Load(env EnvType) {
	call.Call(env, getenv)
	call.Call(env, setenv)
	call.Call(env, unsetenv)
	call.Call(env, chdir)
	call.Call(env, cwd)
	call.Call(env, mkdtemp, 0, 1)
	call.Call(env, remove_all)
	call.Call(env, slurp)
	call.Call(env, slurp_source)
	call.Call(env, spit, 2, 4)

	call.Doc(env, "getenv", "[name]", "Value of the environment variable name, or nil.")
	call.Doc(env, "setenv", "[name value]", "Sets the environment variable name to value.")
	call.Doc(env, "unsetenv", "[name]", "Removes the environment variable name.")
	call.Doc(env, "slurp", "[filename]", "Reads a file and returns its contents as a string.")
	call.Doc(env, "slurp-source", "[filename]", "Reads a source file like slurp and, under lisp-integrity, verifies it against the verified HEAD commit; load-file builds on it.")
	call.Doc(env, "spit", "[filename s & opts]", "Writes string s to a file, creating or truncating it; with :append true, appends instead.")
	call.Doc(env, "chdir", "[path]",
		"Changes the process working directory to path. Process-global: affects the working directory of all subsequent operations, including git repository detection.")
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

// VerifySource is an optional hook that vets a source file before
// load-file (via slurp-source) evaluates it; a non-nil error aborts the
// load. The command package installs it when running under lisp-integrity,
// so code loaded at runtime is verified like the script and its
// requires. slurp itself is never hooked: it reads data, not code.
var VerifySource func(absPath string, content []byte) error

// slurp_source is slurp for files that will be evaluated as code:
// identical, except that under lisp-integrity the content is verified
// against the pinned commit. load-file builds on it.
func slurp_source(fileName string) (MalType, error) {
	v, err := slurp(fileName)
	if err != nil {
		return nil, err
	}
	if VerifySource != nil {
		abs, err := filepath.Abs(fileName)
		if err != nil {
			return nil, err
		}
		if err := VerifySource(abs, []byte(v.(string))); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func slurp(fileName string) (MalType, error) {
	b, e := os.ReadFile(fileName)
	if e != nil {
		return nil, e
	}
	// Register the module so the debugger can map cursors from files
	// loaded at runtime back to their on-disk source (load-file injects
	// a `;; $MODULE <fileName>` prefix naming the module after this same
	// path); without the mapping, stepping would skip those files as
	// library code. Dead code in release builds.
	if lispruntime.Enabled {
		if abs, err := filepath.Abs(fileName); err == nil {
			lispruntime.Modules.Register(fileName, abs)
		}
	}
	return string(b), nil
}

// spit is the write counterpart of slurp, following Clojure's:
// (spit filename s) creates or truncates the file, and
// (spit filename s :append true) appends instead.
func spit(fileName, contents string, opts ...MalType) error {
	if len(opts)%2 != 0 {
		return fmt.Errorf("spit: options must be keyword value pairs")
	}
	appendMode := false
	for i := 0; i < len(opts); i += 2 {
		switch opts[i] {
		case KW("append"):
			b, ok := opts[i+1].(bool)
			if !ok {
				return fmt.Errorf("spit: :append expects a boolean (it was %T)", opts[i+1])
			}
			appendMode = b
		default:
			return fmt.Errorf("spit: unknown option %s", printer.Pr_str(opts[i], true))
		}
	}
	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(fileName, flags, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(contents)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}
