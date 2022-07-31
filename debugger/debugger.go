package debugger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/fatih/color"
	"github.com/jig/lisp"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

type Debugger struct {
	ns        types.EnvType
	name      string
	stop      bool
	trace     bool
	replOnEnd bool
}

func Engine(moduleName string, ns types.EnvType) *Debugger {
	keyboard.Open()
	readConfig()

	printHelp()

	return &Debugger{
		ns:        ns,
		name:      moduleName,
		stop:      true,
		trace:     true,
		replOnEnd: true,
	}
}

func (deb *Debugger) Shutdown() {
	keyboard.Close()

	if deb.replOnEnd {
		colorAlert.Println("Program ended, spawning REPL")
		repl.Execute(context.Background(), deb.ns)
	}
}

var (
	colorFileName  = color.New(color.FgCyan)
	colorSeparator = color.New(color.FgWhite)
	colorPosition  = color.New(color.FgGreen)
	colorAlert     = color.New(color.FgRed)
	colorCode      = color.New(color.FgHiWhite, color.Bold)
	colorKey       = color.New(color.FgHiRed, color.Bold)
	colorDump      = color.New(color.FgYellow)
)

func (deb *Debugger) Stepper(ast types.MalType, ns types.EnvType) {
	expr, ok := ast.(types.List)
	if !ok {
		return
	}
	pos := types.GetPosition(expr)
	if pos != nil && pos.Module != nil && strings.Contains(*pos.Module, deb.name) {
		printTrace(expr, ns, pos, deb.trace)
		if deb.stop {
			for {
				_, key, err := keyboard.GetKey()
				if err != nil {
					return
				}
				switch key {
				case keyboard.KeyF10:
					return
				case keyboard.KeyEnter:
					colorAlert.Println("entering REPL (Enter); use Ctrl+D to exit")
					keyboard.Close()
					// passing ns without a new Env allows debugger to modify it
					repl.Execute(context.Background(), ns)
					keyboard.Open()
					printTrace(expr, ns, pos, deb.trace)
					continue
				case keyboard.KeyF5:
					colorAlert.Println("running to the end (F5)")
					keyboard.Close()
					deb.stop = false
					deb.trace = false
					deb.replOnEnd = false
					return
				case keyboard.KeyF6:
					colorAlert.Println("running to the end and spawn REPL (F6)")
					keyboard.Close()
					deb.stop = false
					deb.trace = false
					deb.replOnEnd = true
					return
				case keyboard.KeyF7:
					colorAlert.Println("running to the end, trace and spawn REPL (F7)")
					keyboard.Close()
					deb.stop = false
					deb.trace = true
					deb.replOnEnd = true
					return
				case keyboard.KeyCtrlC:
					colorAlert.Println("aborting debug session (Ctrl+C)")
					keyboard.Close()
					os.Exit(1)
				case keyboard.KeyEsc:
					// Esc is missfired when repeat pressing an Fn key
					// Ignoring it
					continue
				case 0:
					// 0 means a missfiring when repeated pressing an Fn key
					// Ignoring it
					continue
				default:
					colorAlert.Printf("key %#v not bound\n", key)
				}
			}
		}
	}
}

const dumpFilePath = ".lispdebug/dump-vars.json"

var dump = struct {
	Vars  []string `json:"vars,omitempty"`
	Exprs []string `json:"exprs,omitempty"`
}{}

func readConfig() {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	rawContents, err := os.ReadFile(path.Join(currentUser.HomeDir, dumpFilePath))
	if err != nil {
		return
	}
	if err := json.Unmarshal(rawContents, &dump); err != nil {
		return
	}
}

func saveState() {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	rawContents, err := json.Marshal(dump)
	if err != nil {
		return
	}
	if err := os.MkdirAll(path.Join(currentUser.HomeDir, ".lispdebug"), 0755); err != nil {
		return
	}
	if err := os.WriteFile(path.Join(currentUser.HomeDir, dumpFilePath), rawContents, 0644); err != nil {
		return
	}
}

func printTrace(expr types.MalType, ns types.EnvType, pos *types.Position, trace bool) {
	if trace {
		// actual code trace
		str, _ := lisp.PRINT(expr)
		colorFileName.Print(pos.StringModule())
		colorSeparator.Print("§")
		colorPosition.Println(pos.StringPosition())
		colorCode.Println(str)

		// dump vars
		for _, key := range dump.Vars {
			value, err := ns.Get(types.Symbol{Val: key})
			if err != nil {
				colorAlert.Println(err)
				continue
			}
			switch value.(type) {
			case bool, int, string:
				colorDump.Printf("%s: %v\n", key, value)
			default:
				colorDump.Printf("%s of type %T\n", key, value)
			}
		}

		// dump expressions
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		for _, exprString := range dump.Exprs {
			ast, err := lisp.READ(exprString, types.NewCursorFile("REPL"))
			if err != nil {
				colorAlert.Println(err)
				continue
			}
			res, err := lisp.EVAL(ctx, ast, ns)
			if err != nil {
				colorAlert.Println(err)
				continue
			}
			strRes, err := lisp.PRINT(res)
			if err != nil {
				colorAlert.Println(err)
				continue
			}
			colorCode.Printf("  %s", exprString)
			colorSeparator.Print(": ")
			colorDump.Println(strRes)
		}
	}
}

func printHelp() {
	help := `Debugging session started
  F10:    to execute till next expr
  Enter:  to spawn a REPL on current expr (use Tab to autocomplete)
  F5:     to execute till the end
  F6:     to execute till the end and spawn a REPL
  F7:     to execute till the end, trace expressions and spawn a REPL
  F1:     to execute till the end and trace executed code
  Ctrl+C: to kill this debugging session
`
	for _, line := range strings.Split(help, "\n") {
		strs := strings.Split(line, ":")
		if len(strs) == 2 {
			colorKey.Print(strs[0])
			fmt.Println(strs[1])
		} else {
			fmt.Println(line)
		}
	}
}
