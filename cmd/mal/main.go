package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	goreadline "github.com/chzyer/readline"

	"github.com/jig/mal"
	"github.com/jig/mal/env"
	"github.com/jig/mal/lib/core/nscore"
	"github.com/jig/mal/lib/coreextented/nscoreextended"
	"github.com/jig/mal/lib/test/nstest"

	// "github.com/jig/mal/readline"
	"github.com/jig/mal/types"
)

func main() {
	repl_env, err := env.NewEnv(nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, library := range []struct {
		name string
		load func(repl_env types.EnvType) error
	}{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs},
		{"core mal extended", nscoreextended.Load},
		{"test", nstest.Load},
	} {
		if err := library.load(repl_env); err != nil {
			log.Fatal(err)
		}
	}

	switch len(os.Args) {
	case 0:
		panic("invalid arguments array")
	case 1:
		// repl loop
		ctx := context.Background()
		mal.REPL(repl_env, `(println (str "Mal [" *host-language* "]"))`, &ctx)
		repl(repl_env, &ctx)
		os.Exit(0)
	default:
		if os.Args[1] == "--test" || os.Args[1] == "-t" {
			if len(os.Args) != 3 {
				log.Fatalf("Error: use mal --test <testFiles> to execute test files (%d args)", len(os.Args))
			}
			err = filepath.Walk(os.Args[2], func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					if strings.HasSuffix(info.Name(), "_test.mal") {
						testParams := fmt.Sprintf(`(def! *test-params* {:test-file %q :test-absolute-path %q})`, info.Name(), path)
						if _, err := mal.REPL(repl_env, testParams, nil); err != nil {
							log.Fatalf("Error: %v\n", err)
						}
						if _, err := mal.REPL(repl_env, `(load-file "`+path+`")`, nil); err != nil {
							log.Fatalf("Error: %v\n", err)
						}
					}
				}
				return nil
			})
			if err != nil {
				log.Fatal(err)
			}
			os.Exit(0)
		}

		// called with mal script to load and eval
		ctx := context.Background()
		result, err := mal.REPL(repl_env, `(load-file "`+os.Args[1]+`")`, &ctx)
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
		fmt.Println(result)
	}
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

func repl(repl_env types.EnvType, ctx *context.Context) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		panic(err)
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
		panic(err)
	}
	defer l.Close()

	log.SetOutput(l.Stderr())
	var lines []string
	for {
		line, err := l.Readline()
		if err == goreadline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
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
