package repl

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	goreadline "github.com/chzyer/readline"
	"github.com/jig/mal"
	"github.com/jig/mal/types"
)

func Execute(repl_env types.EnvType, ctx *context.Context) error {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	l, err := goreadline.NewEx(&goreadline.Config{
		Prompt:          "\033[32m»\033[0m ",
		HistoryFile:     dirname + "/.mal_history",
		AutoComplete:    &lispCompleter{repl_env},
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		return err
	}
	defer l.Close()

	log.SetOutput(l.Stderr())
	var lines []string
	for {
		line, err := l.Readline()
		if err == goreadline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			// proper exit with ^D
			return nil
		}

		line = strings.TrimSpace(line)
		lines = append(lines, line)
		completeLine := strings.Join(lines, "\n")
		out, err := mal.REPL(repl_env, completeLine, ctx)
		if err != nil {
			if err.Error() == "expected ')', got EOF" ||
				err.Error() == "expected ']', got EOF" ||
				err.Error() == "expected '}', got EOF" {
				l.SetPrompt("\033[31m›\033[0m ")
				continue
			}
			if err.Error() == "expected '}', got EOF" {
				l.SetPrompt("\033[31m›\033[0m ")
				continue
			}
			if err.Error() == "<empty line>" {
				continue
			}
			lines = []string{}
			l.SetPrompt("\033[32m»\033[0m ")
			fmt.Printf("Error: %v\n", err)
			continue
		}
		lines = []string{}
		l.SetPrompt("\033[32m»\033[0m ")
		fmt.Printf("%v\n", out)
	}
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case goreadline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func listSymbols(repl_env types.EnvType) func(string) []string {
	return func(line string) []string {
		symbols := make([]string, 0)
		repl_env.Map().Range(func(key, value interface{}) bool {
			var toAppend string
			switch value.(type) {
			case types.Func:
				toAppend = "(" + key.(string)
			case types.MalFunc:
				toAppend = "(" + key.(string)
			default:
				toAppend = key.(string)
			}
			symbols = append(symbols, toAppend)
			return true
		})
		symbols = append(symbols, []string{"(do", "(try*", "(if", "(catch*", "(fn*", "(context*", "(macroexpand", "(def!", "(defmacro!", "(let*"}...)
		return symbols
	}
}

type lispCompleter struct {
	repl_env types.EnvType
}

var re = regexp.MustCompile(`[\t\r\n \(\)\[\]\{\}]`)

func (l *lispCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	partial := re.Split(string(line[:pos]), -1)
	lastPartial := partial[len(partial)-1]
	l.repl_env.Map().Range(func(_key, value interface{}) bool {
		key := _key.(string)
		if strings.HasPrefix(key, lastPartial) {
			newLine = append(newLine, []rune(key[len(lastPartial):]))
		}
		return true
	})
	return newLine, len(lastPartial)
}
