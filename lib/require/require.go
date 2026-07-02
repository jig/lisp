// Package require implements a lightweight module loader: the lisp
// function `(require "name")` resolves a module name to a file through
// a search-path cascade and loads it once (via load-file-once, so
// lib/coreextented must be loaded first).
//
// Resolution order for `(require "a/b")` → `a/b.lisp`:
//
//  1. directories given with -i/--include on the command line (or via
//     LoadWithConfig for embedders)
//  2. `.<binary>/` under the enclosing Git repository root
//  3. `$HOME/.config/<binary>/`
//  4. `/usr/local/share/<binary>/`
//
// The library is optional: load it explicitly with nsrequire.Load (or
// require.Load) like any other lib/* namespace.
package require

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/types"
)

//go:embed header-require.lisp
var headerRequire string

// HeaderRequire returns the lisp source of the require function.
func HeaderRequire() string { return headerRequire }

// Config parametrises module resolution.
type Config struct {
	// InterpreterBinary names the per-binary search directories
	// (.<binary>/, ~/.config/<binary>/, /usr/local/share/<binary>/).
	// Empty defaults to "lisp".
	InterpreterBinary string
	// IncludeDirs are searched first, in order.
	IncludeDirs []string
}

var config Config

// Load returns a loader that reads the include directories from the
// command line (-i/--include) and installs `require`/`resolve-require`
// in the environment. binary is used to derive the standard search
// directories.
func Load(binary string) func(types.EnvType) error {
	return func(env types.EnvType) error {
		includeDirs, err := parseIncludeArgs(os.Args[1:])
		if err != nil {
			return err
		}
		return load(env, Config{InterpreterBinary: binary, IncludeDirs: includeDirs})
	}
}

// LoadWithConfig returns a loader with explicit configuration, for
// embedders and tests that do not want os.Args parsed.
func LoadWithConfig(cfg Config) func(types.EnvType) error {
	return func(env types.EnvType) error {
		return load(env, cfg)
	}
}

func load(env types.EnvType, cfg Config) error {
	if strings.TrimSpace(cfg.InterpreterBinary) == "" {
		cfg.InterpreterBinary = "lisp"
	}
	config = cfg

	env.Set(types.Symbol{Val: "*interpreter-binary*"}, cfg.InterpreterBinary)
	call.Call(env, resolve_require)

	if _, err := lisp.REPL(context.Background(), env, headerRequire, types.NewCursorFile("header-require")); err != nil {
		return err
	}
	return nil
}

// parseIncludeArgs extracts -i/--include values from raw command line
// arguments. The require library parses them itself (instead of relying
// on the command package) so it stays loadable by any embedder; the
// command package only declares the flag so the CLI parser accepts it.
func parseIncludeArgs(args []string) ([]string, error) {
	var includes []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		switch arg {
		case "-i", "--include":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("-i/--include requires a value")
			}
			i++
			includes = append(includes, args[i])
			continue
		}
		if strings.HasPrefix(arg, "--include=") {
			includes = append(includes, strings.TrimPrefix(arg, "--include="))
			continue
		}
		if strings.HasPrefix(arg, "-i=") {
			includes = append(includes, strings.TrimPrefix(arg, "-i="))
			continue
		}
	}
	return includes, nil
}

// Resolve maps a module name to the absolute path of its `.lisp` file
// using the same search cascade as the lisp `require` function. It is
// exported for tooling: the LSP server resolves require'd modules
// statically to import their definitions.
func Resolve(module string) (string, error) {
	return resolve_require(module)
}

// resolve_require maps a module name to the absolute path of its
// `.lisp` file, searching the configured roots in order. Module names
// are relative slash-separated paths without extension; absolute paths,
// `..` and hidden segments are rejected.
func resolve_require(module string) (string, error) {
	module = strings.TrimSpace(module)
	if module == "" {
		return "", fmt.Errorf("require: empty module name")
	}
	if strings.ContainsAny(module, " \\:@") {
		return "", fmt.Errorf("require: invalid module path %q", module)
	}
	if strings.HasPrefix(module, "/") || strings.HasPrefix(module, "~") {
		return "", fmt.Errorf("require: invalid module path %q", module)
	}

	clean := path.Clean(module)
	if clean != module || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("require: invalid module path %q", module)
	}

	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("require: invalid module path %q", module)
		}
	}

	fileName := clean + ".lisp"
	for _, root := range requireRoots() {
		if root == "" {
			continue
		}
		candidate := filepath.Join(root, filepath.FromSlash(fileName))
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return candidate, nil
			}
			return abs, nil
		}
	}

	return "", fmt.Errorf("require: module %q not found", module)
}

// requireRoots returns the resolution roots in search order.
func requireRoots() []string {
	var roots []string
	for _, dir := range config.IncludeDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			if abs, err := filepath.Abs(dir); err == nil {
				dir = abs
			}
		}
		roots = append(roots, dir)
	}

	binary := config.InterpreterBinary
	if binary == "" {
		binary = "lisp"
	}

	if gitRoot, ok := findGitRoot(); ok {
		roots = append(roots, filepath.Join(gitRoot, "."+binary))
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".config", binary))
	}
	roots = append(roots, filepath.Join("/usr/local/share", binary))

	return roots
}

// findGitRoot walks up from the working directory looking for a .git
// entry.
func findGitRoot() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
