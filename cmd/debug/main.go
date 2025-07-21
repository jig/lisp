package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jig/lisp"
	"github.com/jig/lisp/debug"
	"github.com/jig/lisp/debugimpl"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/assert/nsassert"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/style"
	"github.com/jig/lisp/types"
	"github.com/jig/scanner"
)

type model struct {
	fileName string
	lines    []string
	message  string

	env types.EnvType

	debugControl chan debug.DebugControl

	cursor         int
	viewFirstLine  int
	screenHeight   int
	screenWidth    int
	totalViewLines int
	posStart       scanner.Position
	posEnd         scanner.Position

	positionString string
	inputString    string
	resultString   string
	errString      string
}

func initialModel(ctrl chan debug.DebugControl) model {
	return model{
		screenHeight: -1,
		screenWidth:  -1,
		debugControl: ctrl,
	}
}

func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("Lisp Debugger")
}

type setCode struct {
	Filename string
	Code     string
}

type endMessage struct {
	Success string
	Err     error
}

// summary returns a summary of the given string, truncated to at most the given width and height.
// If the string is longer than the width, it will be truncated and an ellipsis will
// be added at the end. If the string is longer than the height, it will be truncated
// and the last line will be replaced with an ellipsis.
func summary(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height-1]
		lines = append(lines, "...")
	}
	for i, line := range lines {
		if len(line) > width {
			lines[i] = line[:width-3] + "..."
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setCode:
		m.fileName = msg.Filename
		m.lines = strings.Split(fmt.Sprintf("%s\n\n 📁 :file-name %q\n", msg.Code, msg.Filename), "\n")
		return m, nil
	case endMessage:
		if msg.Err != nil {
			m.message = "Exited with error: " + msg.Err.Error()
		} else {
			m.message = "Exited with Result: " + msg.Success
		}
		return m, nil
	case debug.DebugMessage:
		m.inputString = ""
		m.resultString = ""
		m.errString = ""
		if msg.Contents != nil && msg.Filename != nil {
			m.posStart = scanner.Position{}
			m.posEnd = scanner.Position{}

			m.inputString = ":file-name " + *msg.Filename
			m.resultString = ":file-contents " + *msg.Contents
			return m, func() tea.Msg { return setCode{Code: *msg.Contents, Filename: *msg.Filename} }
		} else {
			pos := types.Pos(msg.Input)
			m.posStart = pos.Start()
			m.posEnd = pos.End()

			if m.posStart.Line-1 < m.viewFirstLine {
				m.viewFirstLine = m.posStart.Line - 1
			}
			if m.posEnd.Line-1 > m.viewFirstLine+m.totalViewLines-4 {
				m.viewFirstLine = m.posEnd.Line - (m.totalViewLines - 4)
			}
			m.env = msg.Env
			m.positionString = fmt.Sprintf(":position %s \"%d:%d-%d:%d\"", m.posStart.Filename, m.posStart.Line, m.posStart.Column, m.posEnd.Line, m.posEnd.Column)
			if msg.Err != nil {
				m.inputString = ":input " + summary(printer.Pr_str(msg.Input, true), 80, 3)
				m.resultString = ""
				m.errString = ":error " + summary(msg.Err.Error(), 80, 3)
			} else if msg.Result != nil {
				m.inputString = ":input " + summary(printer.Pr_str(msg.Input, true), 80, 3)
				m.resultString = ":result " + summary(printer.Pr_str(msg.Result, true), 80, 3)
				m.errString = ""
			} else if msg.Input != nil {
				m.inputString = ":input " + summary(printer.Pr_str(msg.Input, true), 80, 3)
				m.resultString = ""
				m.errString = ""
			} else {
				// this must not happen
				m.message = ":empty-message"
			}
			return m, nil
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.debugControl != nil {
				m.debugControl <- debug.DebugExit
			}
			return m, tea.Quit
		case "esc":
			return m, tea.ClearScreen
		case "f5", "5":
			if m.debugControl != nil {
				m.debugControl <- debug.DebugContinue
			}
			return m, nil
		case "f10", "0":
			if m.debugControl != nil {
				m.debugControl <- debug.DebugStepOver
			}
			return m, nil
		case "f11", "+":
			if m.debugControl != nil {
				m.debugControl <- debug.DebugStepInto
			}
			return m, nil
		case "p":
			if m.debugControl != nil {
				m.debugControl <- debug.DebugStepOut
			}
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.viewFirstLine {
				m.viewFirstLine--
			}
			if m.cursor-m.viewFirstLine > m.totalViewLines-1 {
				m.viewFirstLine++
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.lines)-1 {
				m.cursor++
			}
			if m.cursor > m.viewFirstLine+m.totalViewLines-1 {
				m.viewFirstLine++
			}
			if m.cursor < m.viewFirstLine {
				m.viewFirstLine--
			}
			return m, nil
		case "pgup":
			if m.cursor > 0 {
				m.cursor -= m.totalViewLines
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.viewFirstLine -= m.totalViewLines
				if m.viewFirstLine < 0 {
					m.viewFirstLine = 0
				}
			}
			return m, nil
		case "pgdown":
			if m.cursor < len(m.lines)-1 {
				m.cursor += m.totalViewLines
				if m.cursor > len(m.lines)-1 {
					m.cursor = len(m.lines) - 1
				}
				m.viewFirstLine += m.totalViewLines
				if m.viewFirstLine > len(m.lines)-1 {
					m.viewFirstLine = len(m.lines) - 1
				}
			}
			return m, nil
		case "home":
			m.cursor = 0
			m.viewFirstLine = 0
			return m, nil
		case "end":
			m.cursor = len(m.lines) - 1
			m.viewFirstLine = len(m.lines) - 1
			return m, nil
		default:
			m.message = fmt.Sprintf("Unknown key: %s", msg.String())
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.screenWidth, m.screenHeight = msg.Width, msg.Height
		m.totalViewLines = m.screenHeight / 2
		return m, nil
	default:
		return m, nil
	}
}

// Unicode box drawing characters:
// ┌─┬┐  ╔═╦╗  ╭─┬╮  ▁▂▃▄▅▆▇█  ░▒▓█  ■□▪▫
// │ ││  ║ ║║  │ ││  ▏▎▍▌▋▊▉█  ▓▒░   ●○◆◇
// ├─┼┤  ╠═╬╣  ├─┼┤  ▔▕        ╱╲╳   ◄►▲▼
// └─┴┘  ╚═╩╝  ╰─┴╯  ▏▎▍▌▋▊▉█  ╲╱    ◀▶▴▾
//
// ━┃┏┓┗┛┣┫┳┻╋  ┌┐└┘├

func (m model) View() string {
	if m.screenHeight <= 6 || m.screenWidth <= 16 {
		return "window too small"
	}
	mainHeight := (m.screenHeight * 8 / 10) - 4
	auxHeight := (m.screenHeight - mainHeight) - 4
	mainWidth := (m.screenWidth / 2) - 4
	auxWidth := (m.screenWidth - mainWidth) - 4

	sEnv := showEnv(m.env, auxWidth-20, 0)

	sStdout := m.message
	sResult := m.positionString + "\n" + m.inputString + "\n" + m.resultString + "\n" + m.errString
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().
				Width(mainWidth).Height(mainHeight).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69")).
				Render(
					strings.Trim(style.SyntaxHighlighting(m.viewFirstLine, m.viewFirstLine+mainHeight,
						m.posStart, m.posEnd, strings.Join(m.lines, "\n"), m.cursor), "\n"),
				),
			lipgloss.NewStyle().
				Width(auxWidth).Height(mainHeight).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69")).
				Render(
					strings.Join(strings.Split(strings.Trim(sEnv, "\n"), "\n")[:min(mainHeight, len(strings.Split(strings.Trim(sEnv, "\n"), "\n")))], "\n"),
				)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().
				Width(mainWidth).Height(auxHeight).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69")).
				Render(style.SyntaxHighlighting(0, -1,
					scanner.Position{}, scanner.Position{}, strings.Trim(sResult, "\n"), m.cursor)),
			lipgloss.NewStyle().
				Width(auxWidth).Height(auxHeight).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69")).
				Render(
					strings.Join(strings.Split(strings.Trim(sStdout, "\n"), "\n")[:min(auxHeight, len(strings.Split(strings.Trim(sStdout, "\n"), "\n")))], "\n"),
				),
		),
	)
}

func showEnv(e types.EnvType, width, level int) string {
	if e == nil {
		return ""
	}
	if _, ok := e.(*env.Env); !ok {
		return ""
	}
	// TODO(jig): to be reviewed, this is needed probably due to a bad implementation above
	if e.(*env.Env) == nil {
		return ""
	}
	sEnv := fmt.Sprintf("%d\n", level)
	var newLine [][]rune
	newLine = e.SymbolsOnThisLevel(newLine, "")
	for _, k := range newLine {
		v, err := e.Get(types.Symbol{Val: string(k)})
		if err != nil {
			sEnv += fmt.Sprintf(" %s: %s", string(k), err)
		} else {
			sEnv += "  " + style.Symbol.Render(string(k)) + ": " + style.SyntaxHighlighting(0, -1, scanner.Position{}, scanner.Position{}, printer.Pr_str(v, true)[:min(width, len(printer.Pr_str(v, true)))], -2)
		}
		sEnv += "\n"
	}
	outer := e.Outer()
	if outer != nil {
		return sEnv + showEnv(outer, width, level+1)
	}
	return sEnv
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <file>")
		os.Exit(1)
	}
	filename := os.Args[1]
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("File %s does not exist\n", filename)
		os.Exit(2)
	}

	ctrl := make(chan debug.DebugControl)
	msgs := make(chan debug.DebugMessage)
	getMessage := func(msg debug.DebugMessage) { msgs <- msg }
	dbg := debugimpl.New(getMessage, ctrl)

	p := tea.NewProgram(initialModel(ctrl), tea.WithAltScreen())

	go func() {
		for {
			ns := env.NewEnv()

			for _, library := range []struct {
				name string
				load func(ns types.EnvType) error
			}{
				{"core mal", lisp.LoadNSCore},
				{"core mal with input", lisp.LoadNSCoreInput},
				{"command line args", lisp.LoadNSCoreCmdLineArgs},
				{"concurrent", lisp.LoadNSConcurrent},
				{"core mal extended", func(ns types.EnvType) error { return nscoreextended.Load(ns, dbg) }},
				{"assert", func(ns types.EnvType) error { return nsassert.Load(ns, dbg) }},
				{"system", func(ns types.EnvType) error { return nssystem.Load(ns, dbg) }},
			} {
				if err := library.load(ns); err != nil {
					// log.Fatalf("Library Load Error: %v\n", err)
					p.Send(endMessage{Err: fmt.Errorf("library load(%v) error: %v", filename, err)})
					return
				}
				ns = env.NewSubordinateEnv(ns)
			}

			file, err := os.ReadFile(filename)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)
				os.Exit(3)
			}
			dbg.PushFile(filename, string(file))

			result, err := lisp.REPL(context.Background(), ns, string(file), types.NewCursorHere(filename, 1, 1), dbg)
			if err != nil {
				p.Send(endMessage{Err: fmt.Errorf("Eval(%v) error: %v", filename, err)})
			} else {
				p.Send(endMessage{Success: printer.Pr_str(result, true)})
			}
			dbg.Reset()
		}
	}()

	go func() {
		for msg := range msgs {
			p.Send(msg)
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting terminal: %v", err)
		os.Exit(1)
	}
}
