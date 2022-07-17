package lisp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/core"
	"github.com/jig/lisp/types"
)

func TestFileTests(t *testing.T) {
	dirEntries, err := os.ReadDir("./tests")
	if err != nil {
		log.Fatal(err)
	}
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}
		if !strings.HasSuffix(dirEntry.Name(), ".mal") {
			continue
		}
		if !strings.HasPrefix(dirEntry.Name(), "step") {
			continue
		}
		if dirEntry.Name() == "step0_repl.mal" {
			continue
		}
		if dirEntry.Name() == "step1_read_print.mal" {
			continue
		}
		// fmt.Println(dirEntry.Name())
		code, err := os.ReadFile("./tests/" + dirEntry.Name())
		if err != nil {
			log.Fatal(err)
		}
		if err := parseFile(context.Background(), dirEntry.Name(), string(code)); err != nil {
			log.Fatal(err)
		}
	}
}

func parseFile(ctx context.Context, fileName string, code string) error {
	lines := strings.Split(string(code), "\n")
	currentLine := 0

	env := newEnv(fileName)
	var result types.MalType
	var stdoutResult string
	for _, line := range lines {
		currentLine++
		line = strings.Trim(line, " \t\r\n")
		switch {
		case len(line) == 0:
			continue
		case strings.HasPrefix(line, ";;;"):
			// ignored, all tests executed
			continue
		case strings.HasPrefix(line, ";;"):
			// fmt.Println(line)
			continue
		case strings.HasPrefix(line, ";>>>"):
			// settings/commands ignored
			continue
		case strings.HasPrefix(line, ";=>"):
			line = line[3:]
			if result != line {
				return fmt.Errorf("%q %000d: expected result `%s` got `%s`", fileName, currentLine, line, result)
			}
			continue
		case strings.HasPrefix(line, ";/"):
			line = line[2:]
			matched, err := regexp.MatchString(line, stdoutResult)
			if err != nil {
				return fmt.Errorf("%q %000d: cannot compile regex `%s` got %s", fileName, currentLine, line, err)
			}
			if !matched {
				return fmt.Errorf("%q %000d: expected stdout `%s` got `%s`", fileName, currentLine, line, stdoutResult)
			}
			continue
		case strings.HasPrefix(line, ";"):
			return fmt.Errorf("%q test data error at line %d:\n%s", fileName, currentLine, line)
		default:
			// fmt.Println(currentLine, line)
			result, stdoutResult = captureStdout(func() (types.MalType, error) {
				v, err := REPL(ctx, env, line, types.NewCursorFile(fileName))
				if v == nil {
					return "nil", err
				}
				return v, err
			})
			// fmt.Printf("\t\t%s\t\t\t%s\n", line, stdoutResult)
		}
	}
	return nil
}

func newEnv(fileName string) types.EnvType {
	env, err := env.NewEnv(nil, nil, nil)
	if err != nil {
		panic(err)
	}
	// core.go: defined using go
	for k, v := range core.NS {
		env.Set(types.Symbol{Val: k}, types.Func{Fn: v.(func(context.Context, []types.MalType) (types.MalType, error))})
	}
	for k, v := range core.NSInput {
		env.Set(types.Symbol{Val: k}, types.Func{Fn: v.(func(context.Context, []types.MalType) (types.MalType, error))})
	}
	env.Set(types.Symbol{Val: "eval"}, types.Func{Fn: func(ctx context.Context, a []types.MalType) (types.MalType, error) {
		return EVAL(ctx, a[0], env)
	}})
	env.Set(types.Symbol{Val: "*ARGV*"}, types.List{})

	// package example to test marshalers
	for k, v := range NSMarshalExample {
		env.Set(types.Symbol{Val: k}, types.Func{Fn: v.(func(context.Context, []types.MalType) (types.MalType, error))})
	}

	ctx := context.Background()
	// core.mal: defined using the language itself
	if _, err := REPL(ctx, env, `(def *host-language* "go")`, types.NewCursorFile(fileName)); err != nil {
		return nil
	}
	if _, err := REPL(ctx, env, "(def not (fn (a) (if a false true)))", types.NewCursorFile(fileName)); err != nil {
		return nil
	}
	if _, err := REPL(ctx, env, "(def load-file (fn (f) (eval (read-string (str \"(do \" (slurp f) \"\nnil)\")))))", types.NewCursorFile(fileName)); err != nil {
		return nil
	}
	if _, err := REPL(ctx, env, "(defmacro cond (fn (& xs) (if (> (count xs) 0) (list 'if (first xs) (if (> (count xs) 1) (nth xs 1) (throw \"odd number of forms to cond\")) (cons 'cond (rest (rest xs)))))))", types.NewCursorFile(fileName)); err != nil {
		return nil
	}
	if _, err := REPL(ctx, env, `(def db (atom {}))`, types.NewCursorFile(fileName)); err != nil {
		return nil
	}
	return env
}

func captureStdout(REPL func() (types.MalType, error)) (result types.MalType, stdoutResult string) {
	// see https://stackoverflow.com/questions/10473800/in-go-how-do-i-capture-stdout-of-a-function-into-a-string
	// for the source example an explanation of this Go os.Pipe lines
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w

	result, errREPL := REPL()
	if errREPL != nil {
		switch errREPL := errREPL.(type) {
		case types.MalError:
			errorString, err := PRINT(errREPL.Obj)
			if err != nil {
				fmt.Print("Error: UNPRINTABLE-ERROR")
			} else {
				fmt.Printf("Error: %s", errorString)
			}
		default:
			fmt.Printf("Error: %s", errREPL)
		}
	}

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, err = io.Copy(&buf, r)
		if err != nil {
			log.Println(err)
		}
		outC <- buf.String()
	}()
	w.Close()
	os.Stdout = old
	stdoutResult = <-outC
	return result, stdoutResult
}
