package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

// Execute is the main function of a command line MAL interpreter.
// args are usually the os.Args, and repl_env contains the environment filled
// with the symbols required for the interpreter.
func Execute(args []string, repl_env types.EnvType) error {
	switch len(os.Args) {
	case 0:
		return errors.New("invalid arguments array")
	case 1:
		// repl loop
		ctx := context.Background()
		if _, err := lisp.REPL(ctx, repl_env, `(println (str "Lisp Mal [" *host-language* "]"))`, types.NewCursorFile("REPL")); err != nil {
			return fmt.Errorf("internal error: %s", err)
		}
		if err := repl.Execute(ctx, repl_env); err != nil {
			return err
		}
		return nil
	default:
		if os.Args[1] == "--test" || os.Args[1] == "-t" {
			if len(os.Args) != 3 {
				return fmt.Errorf("use mal --test <testFiles> to execute test files (%d args)", len(os.Args))
			}
			if err := filepath.Walk(os.Args[2], func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					if strings.HasSuffix(info.Name(), "_test.mal") {
						testParams := fmt.Sprintf(`(def *test-params* {:test-file %q :test-absolute-path %q})`, info.Name(), path)

						ctx := context.Background()
						if _, err := lisp.REPL(ctx, repl_env, testParams, types.NewCursorFile(info.Name())); err != nil {
							return err
						}
						if _, err := lisp.REPL(ctx, repl_env, `(load-file "`+path+`")`, types.NewCursorHere(path, -3, 1)); err != nil {
							return err
						}
					}
				}
				return nil
			}); err != nil {
				return err
			}
			return nil
		}

		// called with mal script to load and eval
		result, err := ExecuteFile(os.Args[1], repl_env)
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}
}

// ExecuteFile executes a file on the given path
func ExecuteFile(fileName string, repl_env types.EnvType) (types.MalType, error) {
	ctx := context.Background()
	result, err := lisp.REPL(ctx, repl_env, `(load-file "`+fileName+`")`, types.NewCursorHere(fileName, -3, 1))
	if err != nil {
		return nil, err
	}
	return result, nil
}
