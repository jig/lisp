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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/runtime"
	"github.com/jig/lisp/types"
)

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

func load(rootEnv types.EnvType, cfg Config) error {
	if strings.TrimSpace(cfg.InterpreterBinary) == "" {
		cfg.InterpreterBinary = "lisp"
	}
	config = cfg

	rootEnv.Set(types.Symbol{Val: "*interpreter-binary*"}, cfg.InterpreterBinary)
	call.Call(rootEnv, resolve_require)

	// Each loaded module is evaluated once in its own environment
	// subordinate to rootEnv; the cache lives in this closure so
	// distinct root environments (e.g. tests) do not share modules.
	loader := &moduleLoader{root: rootEnv, cache: map[string]types.EnvType{}}
	rootEnv.Set(types.Symbol{Val: "require"}, types.Func{Fn: loader.require})
	return nil
}

// moduleLoader evaluates modules once and exposes their top-level
// definitions in the root environment under qualified names.
type moduleLoader struct {
	mu    sync.Mutex
	root  types.EnvType
	cache map[string]types.EnvType // abs path → module env
}

// require implements `(require "module" [:as "alias"] [:refer ["name" …]])`.
//
// The module file is evaluated in a fresh environment subordinate to
// the root env, so its internal cross-references keep working (its
// functions close over that environment), and its top-level
// definitions are then published in the root environment as
// `module/name` (or `alias/name` with :as). :refer additionally
// publishes the listed names unqualified. Re-requiring an already
// loaded module does not re-evaluate it, but does publish the new
// alias or refers.
func (l *moduleLoader) require(ctx context.Context, args []types.MalType) (types.MalType, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("require: module name required")
	}
	module, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("require: module name must be a string (was %T)", args[0])
	}
	prefix, refers, err := parseRequireOptions(module, args[1:])
	if err != nil {
		return nil, err
	}

	absPath, err := resolve_require(module)
	if err != nil {
		return nil, err
	}

	moduleEnv, err := l.loadModule(ctx, absPath)
	if err != nil {
		return nil, err
	}

	locals, ok := moduleEnv.(interface{ LocalSymbols() []string })
	if !ok {
		return nil, fmt.Errorf("require: environment does not support symbol enumeration")
	}
	names := locals.LocalSymbols()
	defined := map[string]bool{}
	for _, name := range names {
		v, err := moduleEnv.Get(types.Symbol{Val: name})
		if err != nil {
			continue
		}
		l.root.Set(types.Symbol{Val: prefix + "/" + name}, v)
		defined[name] = true
	}
	for _, name := range refers {
		if !defined[name] {
			return nil, fmt.Errorf("require: symbol '%s' not found in module %q", name, module)
		}
		v, err := moduleEnv.Get(types.Symbol{Val: name})
		if err != nil {
			return nil, err
		}
		l.root.Set(types.Symbol{Val: name}, v)
	}
	return nil, nil
}

// loadModule evaluates the module file once and caches its environment.
func (l *moduleLoader) loadModule(ctx context.Context, absPath string) (types.EnvType, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if moduleEnv, ok := l.cache[absPath]; ok {
		return moduleEnv, nil
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("require: %w", err)
	}
	// Register the module so the debugger maps its cursors back to the
	// on-disk file. Dead code in release builds.
	if runtime.Enabled {
		runtime.Modules.Register(absPath, absPath)
	}
	moduleEnv := env.NewSubordinateEnv(l.root)
	// The `;; $MODULE` prefix names the module in every cursor, exactly
	// like load-file does, so positions in errors and in the debugger
	// point at the real file.
	src := ";; $MODULE " + absPath + "\n(do " + string(content) + "\n)"
	ast, err := lisp.READ(src, nil, moduleEnv)
	if err != nil {
		return nil, err
	}
	if _, err := lisp.EVAL(ctx, ast, moduleEnv); err != nil {
		return nil, err
	}
	l.cache[absPath] = moduleEnv
	return moduleEnv, nil
}

// parseRequireOptions handles the optional `:as "alias"` and
// `:refer ["name" …]` argument pairs. Keywords arrive from the reader
// as strings with the "ʞ" prefix.
func parseRequireOptions(module string, opts []types.MalType) (prefix string, refers []string, err error) {
	prefix = module
	for i := 0; i < len(opts); i += 2 {
		key, ok := opts[i].(string)
		if !ok || !strings.HasPrefix(key, "ʞ") {
			return "", nil, fmt.Errorf("require: expected :as or :refer, got %v", opts[i])
		}
		if i+1 >= len(opts) {
			return "", nil, fmt.Errorf("require: %s requires a value", ":"+strings.TrimPrefix(key, "ʞ"))
		}
		switch strings.TrimPrefix(key, "ʞ") {
		case "as":
			alias, ok := opts[i+1].(string)
			if !ok || alias == "" {
				return "", nil, fmt.Errorf("require: :as expects a non-empty string")
			}
			prefix = alias
		case "refer":
			vec, ok := opts[i+1].(types.Vector)
			if !ok {
				return "", nil, fmt.Errorf("require: :refer expects a vector of strings")
			}
			for _, e := range vec.Val {
				name, ok := e.(string)
				if !ok {
					return "", nil, fmt.Errorf("require: :refer expects a vector of strings")
				}
				refers = append(refers, name)
			}
		default:
			return "", nil, fmt.Errorf("require: unknown option :%s", strings.TrimPrefix(key, "ʞ"))
		}
	}
	return prefix, refers, nil
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

// AddIncludeDirs appends directories to the search cascade at runtime.
// Used by the LSP server to honour editor-configured include
// directories (they land after any -i dirs, before the standard roots).
func AddIncludeDirs(dirs ...string) {
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d != "" {
			config.IncludeDirs = append(config.IncludeDirs, d)
		}
	}
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

	// <BINARY>PATH environment variable (LISPPATH for the default
	// binary), OS path-list separated. Placed after the project's
	// git-root `.lisp/` so a project's own modules always win over
	// whatever the shell env points at — the conservative choice for a
	// variable a user might already have set. The per-binary name keeps
	// it distinctive.
	if envPath := os.Getenv(strings.ToUpper(binary) + "PATH"); envPath != "" {
		for _, dir := range filepath.SplitList(envPath) {
			if dir = strings.TrimSpace(dir); dir != "" {
				roots = append(roots, dir)
			}
		}
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
