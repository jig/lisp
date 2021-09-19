package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mal "github.com/jig/lisp"
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
		mal.REPL(repl_env, `(println (str "Mal [" *host-language* "]"))`, &ctx)
		if err := repl.Execute(repl_env, &ctx); err != nil {
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
						testParams := fmt.Sprintf(`(def! *test-params* {:test-file %q :test-absolute-path %q})`, info.Name(), path)
						if _, err := mal.REPL(repl_env, testParams, nil); err != nil {
							return err
						}
						if _, err := mal.REPLPosition(repl_env, `(load-file "`+path+`")`, nil, &types.Position{
							Module: &path,
							Row:    -3, // "ugly hack: load-file implementation is 4 lines long"
						}); err != nil {
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
		ctx := context.Background()
		result, err := mal.REPLPosition(repl_env, `(load-file "`+os.Args[1]+`")`, &ctx, &types.Position{
			Module: &os.Args[1],
			Row:    -3, // "ugly hack: load-file implementation is 4 lines long"
		})
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}
}
