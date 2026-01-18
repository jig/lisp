package include

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

//go:embed header-include.lisp
var headerInclude string

func HeaderInclude() string { return headerInclude }

type RequireConfig struct {
	InterpreterBinary string
	IncludeDirs       []string
}

var config RequireConfig

func Load(binary string) func(types.EnvType) error {
	return func(env types.EnvType) error {
		if strings.TrimSpace(binary) == "" {
			binary = "lisp"
		}
		config.InterpreterBinary = binary

		includeDirs, err := parseIncludeArgs(os.Args[1:])
		if err != nil {
			return err
		}
		config.IncludeDirs = includeDirs

		env.Set(types.Symbol{Val: "*interpreter-binary*"}, binary)
		call.Call(env, resolve_require)

		if _, err := lisp.REPL(context.Background(), env, headerInclude, types.NewCursorFile("$include")); err != nil {
			return err
		}
		return nil
	}
}

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
