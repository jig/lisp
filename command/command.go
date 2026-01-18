package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/jig/lisp"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

// args represents command line arguments for the Lisp interpreter
type args struct {
	Version bool     `arg:"-v,--version" help:"show version information"`
	Test    string   `arg:"-t,--test" help:"run test suite from directory" placeholder:"DIR"`
	Debug   bool     `arg:"--debug" help:"enable DEBUG-EVAL support (may impact performance)"`
	Eval    string   `arg:"-e,--eval" help:"evaluate expression and exit" placeholder:"EXPR"`
	Include []string `arg:"-i,--include,separate" help:"add include directory for require" placeholder:"DIR"`
	Script  string   `arg:"positional" help:"lisp script to execute"`
	Args    []string `arg:"positional" help:"arguments to pass to the script"`
}

func (args) Description() string {
	return "Lisp interpreter"
}

// PreParseArgs does a preliminary parse of arguments to extract the script arguments
// before libraries are loaded. This is needed because if LoadCmdLineArgs is used it
// needs to know the script arguments before the main Execute runs.
func PreParseArgs(cmdArgs []string) []string {
	ordering := evalOrderingFromArgs(cmdArgs[1:])
	if ordering.EvalSeen {
		if ordering.Script != "" {
			return ordering.ScriptArgs
		}
		return ordering.EvalArgs
	}

	var parsedArgs args
	parser, err := arg.NewParser(arg.Config{Program: "lisp"}, &parsedArgs)
	if err != nil {
		// Silently ignore parsing errors at this stage
		return []string{}
	}

	// Parse arguments (skip program name)
	if len(cmdArgs) > 1 {
		err = parser.Parse(cmdArgs[1:])
		if err != nil {
			// Silently ignore parsing errors at this stage
			return []string{}
		}
	}

	// Set scriptArgs from parsed Args (arguments after script name)
	return parsedArgs.Args
}

// Execute is the main function of a command line MAL interpreter.
// args are usually the os.Args, and repl_env contains the environment filled
// with the symbols required for the interpreter.
func Execute(cmdArgs []string, repl_env types.EnvType) error {
	var parsedArgs args
	parser, err := arg.NewParser(arg.Config{Program: "lisp"}, &parsedArgs)
	if err != nil {
		return err
	}

	ordering := evalOrderingFromArgs(cmdArgs[1:])

	// Parse arguments (skip program name)
	if len(cmdArgs) > 1 {
		err = parser.Parse(cmdArgs[1:])
		if err == arg.ErrHelp {
			parser.WriteHelp(os.Stdout)
			return nil
		}
		if err != nil {
			return err
		}
	}

	// Enable DEBUG-EVAL if flag is set
	if parsedArgs.Debug {
		lisp.DebugEvalEnabled = true
	}

	if ordering.EvalSeen {
		if ordering.Script != "" {
			parsedArgs.Script = ordering.Script
			parsedArgs.Args = ordering.ScriptArgs
			setArgv(repl_env, ordering.ScriptArgs)
		} else {
			parsedArgs.Script = ""
			parsedArgs.Args = ordering.EvalArgs
			setArgv(repl_env, ordering.EvalArgs)
		}
	}

	if parsedArgs.Eval != "" && (parsedArgs.Version || parsedArgs.Test != "") {
		return fmt.Errorf("-e cannot be used with --version or --test")
	}

	// Handle --version
	if parsedArgs.Version {
		versionInfo, ok := debug.ReadBuildInfo()
		if !ok {
			fmt.Println("Lisp version information unavailable")
			return nil
		}
		fmt.Printf("Lisp:\n%s\n", versionInfo)
		return nil
	}

	// Handle --test
	if parsedArgs.Test != "" {
		return runTests(parsedArgs.Test, repl_env)
	}

	// Handle file execution or stdin
	if parsedArgs.Script != "" {
		// Special case: "-" means read from stdin (like Python/Ruby)
		if parsedArgs.Script == "-" {
			// Execute from stdin (interactive mode becomes REPL)
			ctx := context.Background()
			if _, err := lisp.REPL(ctx, repl_env, `(println (str "Lisp Mal [" *host-language* "]"))`, types.NewCursorFile("REPL")); err != nil {
				return fmt.Errorf("internal error: %s", err)
			}
			if err := repl.Execute(ctx, repl_env); err != nil {
				return err
			}
		} else {
			// Execute file
			result, err := ExecuteFile(parsedArgs.Script, repl_env)
			if err != nil {
				return err
			}
			if parsedArgs.Eval == "" {
				fmt.Println(result)
			}
		}

		if parsedArgs.Eval == "" {
			return nil
		}
	}

	if parsedArgs.Eval != "" {
		if ordering.EvalSeen {
			setArgv(repl_env, ordering.EvalArgs)
		}
		ctx := context.Background()
		result, err := lisp.REPL(ctx, repl_env, parsedArgs.Eval, types.NewCursorFile("-e"))
		if err != nil {
			return err
		}
		fmt.Println(result)
		return nil
	}

	// Default: start REPL
	ctx := context.Background()
	if _, err := lisp.REPL(ctx, repl_env, `(println (str "Lisp Mal [" *host-language* "]"))`, types.NewCursorFile("REPL")); err != nil {
		return fmt.Errorf("internal error: %s", err)
	}
	return repl.Execute(ctx, repl_env)
}

// runTests executes all *_test.mal files in the given directory
func runTests(dir string, repl_env types.EnvType) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), "_test.mal") {
			testParams := fmt.Sprintf(`(def *test-params* {:test-file %q :test-absolute-path %q})`, info.Name(), path)

			ctx := context.Background()
			if _, err := lisp.REPL(ctx, repl_env, testParams, types.NewCursorFile(info.Name())); err != nil {
				return err
			}
			if _, err := lisp.REPL(ctx, repl_env, `(load-file "`+path+`")`, types.NewCursorHere(path, -3, 1)); err != nil {
				return err
			}
		}
		return nil
	})
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

type evalOrdering struct {
	EvalSeen   bool
	Script     string
	ScriptArgs []string
	EvalArgs   []string
}

func evalOrderingFromArgs(cmdArgs []string) evalOrdering {
	var before []string
	var after []string
	var evalSeen bool
	stopFlags := false

	for i := 0; i < len(cmdArgs); i++ {
		arg := cmdArgs[i]

		if !stopFlags {
			if arg == "--" {
				stopFlags = true
				continue
			}

			switch arg {
			case "-e", "--eval":
				evalSeen = true
				if i+1 < len(cmdArgs) {
					i++
				}
				continue
			case "-i", "--include", "-t", "--test":
				if i+1 < len(cmdArgs) {
					i++
				}
				continue
			case "-v", "--version", "--debug":
				continue
			}

			if strings.HasPrefix(arg, "--eval=") || strings.HasPrefix(arg, "-e=") {
				evalSeen = true
				continue
			}
			if strings.HasPrefix(arg, "--include=") || strings.HasPrefix(arg, "-i=") || strings.HasPrefix(arg, "--test=") {
				continue
			}
			if strings.HasPrefix(arg, "-") {
				continue
			}
		}

		if evalSeen {
			after = append(after, arg)
		} else {
			before = append(before, arg)
		}
	}

	var script string
	var scriptArgs []string
	if len(before) > 0 {
		script = before[0]
		if len(before) > 1 {
			scriptArgs = append(scriptArgs, before[1:]...)
		}
	}

	return evalOrdering{
		EvalSeen:   evalSeen,
		Script:     script,
		ScriptArgs: scriptArgs,
		EvalArgs:   after,
	}
}

func setArgv(env types.EnvType, args []string) {
	if len(args) == 0 {
		env.Set(types.Symbol{Val: "*ARGV*"}, types.List{})
		return
	}
	list := types.List{Val: make([]types.MalType, 0, len(args))}
	for _, a := range args {
		list.Val = append(list.Val, a)
	}
	env.Set(types.Symbol{Val: "*ARGV*"}, list)
}
