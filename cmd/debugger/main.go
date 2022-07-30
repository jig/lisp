package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/eiannone/keyboard"
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

func stepper(moduleName string) func(ast types.MalType, ns types.EnvType) {
	help := `Keys Cheat Sheet:
	F10 to execute till next expr
	Enter to spawn a REPL on current expr
	F5 to execute till the end of the program
	Esc or ^C to kill this debugging session
`
	fmt.Println(help)

	stop := true
	return func(ast types.MalType, ns types.EnvType) {
		if !stop {
			return
		}
		expr, ok := ast.(types.List)
		if !ok {
			return
		}
		pos := types.GetPosition(expr)
		if pos != nil && pos.Module != nil && strings.Contains(*pos.Module, moduleName) {
			str, _ := lisp.PRINT(expr)
			fmt.Printf("--- [%d]%s ---\n%s\n", ns.Deepness(), pos, str)

			for {
				_, key, err := keyboard.GetKey()
				if err != nil {
					panic(err)
				}
				switch key {
				case keyboard.KeyF10:
					return
				case keyboard.KeyEnter:
					repl.Execute(context.Background(), ns)
					return
				case keyboard.KeyF5:
					keyboard.Close()
					stop = false
					return
				case keyboard.KeyEsc, keyboard.KeyCtrlC:
					keyboard.Close()
					fmt.Println("debug session aborted")
					os.Exit(1)
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
}
