package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/eiannone/keyboard"
	"github.com/fatih/color"
	"github.com/jig/lisp"
	"github.com/jig/lisp/command"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/repl"
	"github.com/jig/lisp/types"
)

var replOnEnd bool

var (
	colorFileName  = color.New(color.FgCyan)
	colorSeparator = color.New(color.FgWhite)
	colorPosition  = color.New(color.FgGreen)
	colorAlert     = color.New(color.FgRed)
	colorCode      = color.New(color.FgHiWhite, color.Bold)
	colorKey       = color.New(color.FgHiRed, color.Bold)
)

func stepper(moduleName string) func(ast types.MalType, ns types.EnvType) {
	printHelp()

	stop := true
	trace := true
	return func(ast types.MalType, ns types.EnvType) {
		expr, ok := ast.(types.List)
		if !ok {
			return
		}
		pos := types.GetPosition(expr)
		if pos != nil && pos.Module != nil && strings.Contains(*pos.Module, moduleName) {
			printTrace(expr, pos, trace)
			if stop {
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
						printTrace(expr, pos, trace)
						continue
					case keyboard.KeyF5:
						colorAlert.Println("running to the end (F5)")
						keyboard.Close()
						stop = false
						trace = false
						replOnEnd = false
						return
					case keyboard.KeyF6:
						colorAlert.Println("running to the end and spawn REPL (F6)")
						keyboard.Close()
						stop = false
						trace = false
						replOnEnd = true
						return
					case keyboard.KeyF7:
						colorAlert.Println("running to the end, trace and spawn REPL (F7)")
						keyboard.Close()
						stop = false
						trace = true
						replOnEnd = true
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

func printTrace(expr types.MalType, pos *types.Position, trace bool) {
	if trace {
		str, _ := lisp.PRINT(expr)
		colorFileName.Print(pos.StringModule())
		colorSeparator.Print("§")
		colorPosition.Println(pos.StringPosition())
		colorCode.Println(str)
	}
}

func printHelp() {
	help := `During debugging session
	F10:    to execute till next expr
	Enter:  to spawn a REPL on current expr (on REPL you can use Tab for autocomplete symbols on current namespace)
	F5:     to execute till the end of the program
	F6:     to execute till the end of the program and spawn a REPL in existing environment
	F7:     to execute till the end of the program, trace all expressions and spawn a REPL in existing environment
	F1:     to execute till the end of the program and trace executed code
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

func main() {
	ns := env.NewEnv()

	for _, library := range []struct {
		name string
		load func(ns types.EnvType) error
	}{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs},
		{"core mal extended", nscoreextended.Load},
		{"test", nstest.Load},
		{"concurrent", nsconcurrent.Load},
	} {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	keyboard.Open()
	// defer keyboard.Close()

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <file>\n", os.Args[0])
	}
	lisp.Stepper = stepper(os.Args[1])

	if err := command.Execute(os.Args, ns); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	keyboard.Close()

	if replOnEnd {
		fmt.Println("Program ended")
		repl.Execute(context.Background(), ns)
	}
}
