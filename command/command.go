package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/jig/lisp"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

func printHelp() {
	fmt.Println(`Lisp
	--version, -v provides the version number
	--help, -h provides this help message
	--test, -t runs the test suite
	--debug enables DEBUG-EVAL support (may impact performance)`)
}

// Execute is the main function of a command line MAL interpreter.
// args are usually the os.Args, and repl_env contains the environment filled
// with the symbols required for the interpreter.
func Execute(args []string, repl_env types.EnvType) error {
	// Parse flags
	var debugEval bool
	flagSet := flag.NewFlagSet("lisp", flag.ContinueOnError)
	flagSet.BoolVar(&debugEval, "debug", false, "Enable DEBUG-EVAL support")
	_ = flagSet.Parse(os.Args[1:])

	// Enable DEBUG-EVAL if flag is set
	if debugEval {
		lisp.DebugEvalEnabled = true
	}

	// Get remaining args after flag parsing
	remainingArgs := flagSet.Args()
	argsCount := len(remainingArgs) + 1 // +1 for program name

	switch argsCount {
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
		if len(remainingArgs) == 0 {
			return errors.New("no arguments provided")
		}
		switch remainingArgs[0] {
		case "--version", "-v":
			versionInfo, ok := debug.ReadBuildInfo()
			if !ok {
				fmt.Println("Lisp versions error")
				return nil
			}
			fmt.Printf("Lisp versions:\n%s\n", versionInfo)
			return nil
		case "--help", "-h":
			printHelp()
			return nil
		case "--test", "-t":
			if len(remainingArgs) != 2 {
				printHelp()
				return fmt.Errorf("too many args")
			}
			if err := filepath.Walk(remainingArgs[1], func(path string, info os.FileInfo, err error) error {
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
		result, err := ExecuteFile(remainingArgs[0], repl_env)
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}
}

// ExecuteFile executes a file on the given path
func ExecuteFile(fileName string, ns types.EnvType) (types.MalType, error) {
	ctx := context.Background()
	result, err := lisp.REPL(ctx, ns, `(load-file "`+fileName+`")`, types.NewCursorHere(fileName, -3, 1))
	if err != nil {
		return nil, err
	}
	return result, nil
}
