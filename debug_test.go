package lisp

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/jig/lisp/debug"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/types"
)

var debugStack []string

func TestDebug(t *testing.T) {
	ns := env.NewEnv()

	for _, library := range []struct {
		name string
		load func(ns types.EnvType) error
	}{
		{"core mal", LoadNSCore},
		{"core mal with input", LoadNSCoreInput},
		{"command line args", LoadNSCoreCmdLineArgs},
		{"concurrent", LoadNSConcurrent},
		// {"core mal extended", nscoreextended.Load},
		// {"assert", nsassert.Load},
		// {"system", nssystem.Load},
	} {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
			return
		}
	}
	tests := []struct {
		input    string
		expected string
		stack    string
	}{
		{"()", "()", "[TestDebug:0 -> () 🏳️ () ○ () 🚩]"},
		{"(+ 1 2)", "3", "[TestDebug:1 -> (+ 1 2) 🏳️ (+ 1 2) ○ 3 🚩]"},
		{"(+ 1 2) ;; hola", "3", "[TestDebug:2 -> (+ 1 2) ;; hola 🏳️ (+ 1 2) ○ 3 🚩]"},
		// {"(+ 1 2) (- 10 1) ;; hola", "9", "[TestDebug:3 -> (+ 1 2) (- 10 1) ;; hola 🏳️ (+ 1 2) ○ 3 🚩 (- 10 1) ○ 9 🚩]"},
		// {
		// 	`(load-file "./examples/simple.lisp") (def a (+ x 1)) (def b (+ a 2))`,
		// 	"16",
		// 	"[TestDebug:4 -> (load-file \"./examples/simple.lisp\") (def a (+ x 1)) (def b (+ a 2)) 🏳️ (load-file \"./examples/simple.lisp\") ○ 13 🚩 (def a (+ x 1)) ○ 14 🚩 (def b (+ a 2)) ○ 16 🚩]",
		// },
	}

	for i, test := range tests {
		dbg := &DebugTestType{}

		filename := fmt.Sprintf("%s:%d", t.Name(), i)
		dbg.PushFile(filename, test.input)
		result, err := REPL(context.Background(), ns, test.input, types.NewCursorHere(filename, 1, 1), dbg)
		if err != nil {
			t.Errorf("Eval(%v) error: %v", test.input, err)
			continue
		}

		if test.expected != result {
			t.Fatalf("Eval(%v) = %#v; expected %#v", test.input, result, test.expected)
		}
		if fmt.Sprint(debugStack) != test.stack {
			t.Fatalf("Failed %s != %s", fmt.Sprint(debugStack), test.stack)
		}
		dbg.Reset()
	}
}

type DebugTestType struct{}

// CancelStatus implements DebugEval.
func (d *DebugTestType) CancelStatus() bool {
	return false
}

// Reset implements DebugEval.
func (d *DebugTestType) Reset() {
	debugStack = []string{}
}

func (dbg *DebugTestType) Wait(msg debug.DebugMessage) debug.DebugControl {
	if msg.Result != nil {
		debugStack = append(debugStack, printer.Pr_str(msg.Result, true), "🚩")
	} else if msg.Input != nil {
		debugStack = append(debugStack, printer.Pr_str(msg.Input, true), "○")
	} else {
		debugStack = append(debugStack, "!!", "??")
	}
	return debug.DebugStepOver
}

func (*DebugTestType) DoNotStopStatus() bool { return false }

func (*DebugTestType) SetDoNotStop(bool) bool { return false }

func (*DebugTestType) SetCancelStatus(bool) bool { return false }

func (*DebugTestType) PushFile(filename, contents string) {
	debugStack = append(debugStack, filename, "->")
	debugStack = append(debugStack, contents, "🏳️")
}

func (*DebugTestType) File(filename string) (contents string, exists bool) { return }
