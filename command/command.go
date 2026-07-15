package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/alexflint/go-arg"
	"github.com/jig/lisp"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

// args represents command line arguments for the Lisp interpreter
type args struct {
	Version   bool     `arg:"-v,--version" help:"show version information"`
	Test      string   `arg:"-t,--test" help:"run the test suite from a directory or a single test file" placeholder:"DIR|FILE"`
	TestJSON  string   `arg:"--test-json" help:"with --test, also write a JSON report to the given file" placeholder:"FILE"`
	Coverage  string   `arg:"--coverage" help:"write an lcov coverage report of the executed lisp code (requires lispdebug build)" placeholder:"FILE"`
	Debug     bool     `arg:"--debug" help:"enable DEBUG-EVAL support (requires lispdebug build)"`
	Eval      string   `arg:"-e,--eval" help:"evaluate expression and exit" placeholder:"EXPR"`
	Fmt       bool     `arg:"--fmt" help:"format lisp source (files given as arguments, or stdin) and print the result"`
	Write     bool     `arg:"-w,--write" help:"with --fmt, rewrite each file in place instead of printing"`
	Include   []string `arg:"-i,--include,separate" help:"add include directory for require (needs the require library loaded)" placeholder:"DIR"`
	Preamble  []string `arg:"-P,--preamble,separate" help:"define a preamble placeholder for the script, e.g. -P '$NAME <expr>'" placeholder:"ASSIGN"`
	DAP       bool     `arg:"--dap" help:"start a Debug Adapter Protocol server on stdio (requires lispdebug build)"`
	DAPListen string   `arg:"--dap-listen" help:"start a DAP server on the given TCP address (requires lispdebug build)" placeholder:"HOST:PORT"`
	LSP       bool     `arg:"--lsp" help:"start a Language Server Protocol server on stdio (requires lispdebug build)"`
	LSPListen string   `arg:"--lsp-listen" help:"start an LSP server on the given TCP address (requires lispdebug build)" placeholder:"HOST:PORT"`
	Script    string   `arg:"positional" help:"lisp script to execute"`
	Args      []string `arg:"positional" help:"arguments to pass to the script"`
}

func (args) Description() string {
	return "Lisp interpreter"
}

// PreParseArgs does a preliminary parse of arguments to extract the script arguments
// before libraries are loaded. This is needed because if LoadCmdLineArgs is used it
// needs to know the script arguments before the main Execute runs.
func PreParseArgs(cmdArgs []string) []string {
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

	// Enable DEBUG-EVAL if flag is set. setupDebugHook is gated by the
	// `lispdebug` build tag: in release builds it returns an error so the
	// flag cannot accidentally enable hook code that was compiled out.
	if parsedArgs.Debug {
		if err := setupDebugHook(); err != nil {
			return err
		}
	}

	if parsedArgs.Eval != "" && (parsedArgs.Version || parsedArgs.Test != "") {
		return fmt.Errorf("-e cannot be used with --version or --test")
	}

	// Formatting is a standalone text transformation; it loads no libraries
	// and runs no code, so handle it before any of the evaluating modes.
	if parsedArgs.Fmt {
		files := []string{}
		if parsedArgs.Script != "" {
			files = append(files, parsedArgs.Script)
		}
		files = append(files, parsedArgs.Args...)
		return formatSources(files, parsedArgs.Write)
	}

	// DAP server takes precedence over the rest of the modes when set.
	if parsedArgs.DAP || parsedArgs.DAPListen != "" {
		return startDAP(parsedArgs.DAPListen, parsedArgs.Script, parsedArgs.Preamble, repl_env)
	}

	// LSP server likewise runs instead of the normal modes.
	if parsedArgs.LSP || parsedArgs.LSPListen != "" {
		return startLSP(parsedArgs.LSPListen, repl_env)
	}

	// Handle --version
	if parsedArgs.Version {
		lispVer, scannerVer, _ := core.Versions()
		if lispVer == "" {
			lispVer = "(unknown)"
		}
		if scannerVer == "" {
			scannerVer = "(unknown)"
		}
		fmt.Printf("jig/lisp    %s\n", lispVer)
		fmt.Printf("jig/scanner %s\n", scannerVer)

		versionInfo, ok := debug.ReadBuildInfo()
		if !ok {
			return nil
		}
		fmt.Println(versionInfo)
		return nil
	}

	// Coverage collection wraps the evaluating modes (--test and script
	// execution). In release builds startCoverage returns an error when a
	// coverage file is requested.
	stopCoverage := func() error { return nil }
	if parsedArgs.Coverage != "" {
		stopCoverage, err = startCoverage(parsedArgs.Coverage)
		if err != nil {
			return err
		}
	}

	// Handle --test
	if parsedArgs.Test != "" {
		testErr := runTests(parsedArgs.Test, parsedArgs.TestJSON, repl_env)
		if err := stopCoverage(); err != nil {
			return err
		}
		return testErr
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
			result, err := runScript(context.Background(), repl_env, parsedArgs.Script, parsedArgs.Preamble,
				types.NewCursorHere(parsedArgs.Script, -3, 1))
			if err != nil {
				return err
			}
			if err := stopCoverage(); err != nil {
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

// ExecuteFile executes a file on the given path
func ExecuteFile(fileName string, ns types.EnvType) (types.MalType, error) {
	if abs, err := filepath.Abs(fileName); err == nil {
		registerModule(fileName, abs)
	}
	ctx := context.Background()
	result, err := lisp.REPL(ctx, ns, `(load-file "`+fileName+`")`, types.NewCursorHere(fileName, -3, 1))
	if err != nil {
		return nil, err
	}
	return result, nil
}
