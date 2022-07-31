package debugger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
	"sort"
	"strings"
	"time"

	goreadline "github.com/chzyer/readline"
	"github.com/eiannone/keyboard"
	"github.com/fatih/color"
	"github.com/jig/lisp"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

type Debugger struct {
	config    DebuggerConfig
	ns        types.EnvType
	name      string
	stop      bool
	trace     bool
	replOnEnd bool
}

const dumpFilePath = ".lispdebug/dump-vars.json"

type DebuggerConfig struct {
	Exprs map[string]bool `json:"exprs"`
}

func Engine(moduleName string, ns types.EnvType) *Debugger {
	this := &Debugger{
		ns:        ns,
		name:      moduleName,
		stop:      true,
		trace:     true,
		replOnEnd: true,
	}
	readConfig(this)

	printHelp()
	keyboard.Open()
	return this
}

func (deb *Debugger) Shutdown() {
	keyboard.Close()

	if deb.replOnEnd {
		colorAlert.Println("Program ended, spawning REPL")
		repl.Execute(context.Background(), deb.ns)
	}
	saveState(deb)
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

func (deb *Debugger) Stepper(ast types.MalType, ns types.EnvType, isMacro bool) {
	expr, ok := ast.(types.List)
	if !ok {
		return
	}
	pos := types.GetPosition(expr)
	if pos != nil && pos.Module != nil && strings.Contains(*pos.Module, deb.name) {
		deb.printTrace(expr, ns, pos, isMacro)
		if deb.stop {
			for {
				rune, key, err := keyboard.GetKey()
				if err != nil {
					return
				}
				if rune != 0 {
					switch rune {
					case '+':
						colorAlert.Println("add a new watch (+)")
						line, err := varREPL().Readline()
						if err != nil {
							break
						}
						line = strings.Trim(line, " \t\n\r")
						if len(line) == 0 {
							break
						}
						deb.config.Exprs[line] = true
					case '-':
						colorAlert.Println("removing a new watch (-)")
						line, err := varREPL().Readline()
						if err != nil {
							break
						}
						line = strings.Trim(line, " \t\n\r")
						if len(line) == 0 {
							break
						}
						if _, ok := deb.config.Exprs[line]; ok {
							delete(deb.config.Exprs, line)
						} else {
							colorAlert.Printf("watch %s unexistent\n")
						}
					default:
						colorAlert.Printf("key '%c' not bound\n", rune)
					}
					deb.printTrace(expr, ns, pos, isMacro)
				} else {
					switch key {
					case keyboard.KeyF10:
						return
					case keyboard.KeyEnter:
						colorAlert.Println("entering REPL (Enter); use Ctrl+D to exit")
						keyboard.Close()
						// passing ns without a new Env allows debugger to modify it
						repl.Execute(context.Background(), ns)
						keyboard.Open()
						deb.printTrace(expr, ns, pos, isMacro)
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
}

func readConfig(deb *Debugger) {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if deb.config.Exprs == nil {
			deb.config.Exprs = make(map[string]bool)
		}
	}()

	rawContents, err := os.ReadFile(path.Join(currentUser.HomeDir, dumpFilePath))
	if err != nil {
		return
	}
	if err := json.Unmarshal(rawContents, &deb.config); err != nil {
		return
	}
}

func saveState(deb *Debugger) {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	rawContents, err := json.Marshal(deb.config)
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

func (deb *Debugger) printTrace(expr types.MalType, ns types.EnvType, pos *types.Position, isMacro bool) {
	if deb.trace {
		// dump expressions
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		exprsSorted := []string{}
		for exprString, _ := range deb.config.Exprs {
			exprsSorted = append(exprsSorted, exprString)
		}
		sort.Strings(exprsSorted)
		for _, exprString := range exprsSorted {
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
			colorCode.Print(exprString)
			colorSeparator.Print(": ")
			colorDump.Println(strRes)
		}

		// actual code trace
		str, _ := lisp.PRINT(expr)
		if isMacro {
			colorSeparator.Print("M")
			colorCode.Println(str)
		} else {
			colorCode.Println(str)
		}

		colorFileName.Print(pos.StringModule())
		colorSeparator.Print("§")
		colorPosition.Println(pos.StringPosition())
		fmt.Println()
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
  +:      to add a new expression to dump view
  -:      to remove a expression from dump view
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

func varREPL() *goreadline.Instance {
	l, err := goreadline.NewEx(&goreadline.Config{
		Prompt:          "\033[32m›\033[0m ",
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		log.Fatal(err)
	}
	return l
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case goreadline.CharCtrlZ:
		return r, false
	}
	return r, true
}