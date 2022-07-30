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
)

func stepper(moduleName string) func(ast types.MalType, ns types.EnvType) {
	help := `During debugging session:
	F10:    to execute till next expr
	Enter:  to spawn a REPL on current expr (on REPL you can use Tab for autocomplete symbols on current namespace)
	F5:     to execute till the end of the program
	Esc:    to execute till the end of the program and spawn a REPL in existing environment
	F1:     to execute till the end of the program and trace executed code
	Ctrl+C: to kill this debugging session
`
	fmt.Println(help)

	stop := true
	trace := true
	return func(ast types.MalType, ns types.EnvType) {
		expr, ok := ast.(types.List)
		if !ok {
			return
		}
		pos := types.GetPosition(expr)
		if pos != nil && pos.Module != nil && strings.Contains(*pos.Module, moduleName) {
			if trace {
				str, _ := lisp.PRINT(expr)
				colorFileName.Print(pos.StringModule())
				colorSeparator.Print("§")
				colorPosition.Println(pos.StringPosition())
				fmt.Println(str)
			}
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
						repl.Execute(context.Background(), ns)
						return
					case keyboard.KeyF5:
						colorAlert.Println("running to the end (F5)")
						keyboard.Close()
						stop = false
						trace = false
						replOnEnd = false
						return
					case keyboard.KeyF1:
						colorAlert.Println("running to the end (F1)")
						keyboard.Close()
						stop = false
						trace = true
						replOnEnd = false
						return
					case keyboard.KeyEsc:
						keyboard.Close()
						stop = false
						trace = false
						replOnEnd = true
						return
					case keyboard.KeyCtrlC:
						keyboard.Close()
						fmt.Println("debug session aborted")
						os.Exit(1)
					default:
						colorAlert.Printf("key %s not bound\n")
					}
				}
			}
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
