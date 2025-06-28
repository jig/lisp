package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jig/scanner"
)

// SyntaxHighlighting highlights the code visible on a window and returns it in a string.
// It takes the following parameters:
// - firstLine: the first line to be rendered
// - lastLine: the last line to be rendered
// - start: the starting position of the code
// - end: the ending position of the code
// - code: the code to be rendered
// - cursor: the position of the line cursor
// - breakpoints: a slice of booleans indicating the lines with breakpoints
// It returns the highlighted code as a string.
func SyntaxHighlighting(
	// lines to be rendered
	firstLine, lastLine int,
	start, end scanner.Position,
	// code to be rendered
	code string,
	cursor int,
) string {
	result := syntaxHighlightingAll(start, end, code)
	codeLines := strings.Split(result, "\n")
	for i := range codeLines {
		prefix := ""
		if i == cursor {
			prefix += lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Render("▶")
		} else {
			prefix += lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render("·")
		}
		codeLines[i] = prefix + codeLines[i]
	}

	// code below supposes that highlighted code escape characters do not introduce additional \n characters
	if lastLine < 0 || lastLine >= len(codeLines) {
		if firstLine < 0 {
			return strings.Join(codeLines, "\n")
		} else if firstLine >= len(codeLines) {
			return strings.Join([]string{}, "\n")
		} else {
			return strings.Join(codeLines[firstLine:], "\n")
		}
	} else {
		if firstLine < 0 {
			return strings.Join(codeLines[:lastLine], "\n")
		} else if firstLine >= len(codeLines) {
			return strings.Join([]string{}, "\n")
		} else {
			return strings.Join(codeLines[firstLine:lastLine], "\n")
		}
	}
}

// nolint: gocyclo
// syntaxHighlightingAll highlights the code using the scanner package.
// It returns the highlighted code as a string.
func syntaxHighlightingAll(
	start, end scanner.Position,
	// code to be rendered
	code string,
) string {
	s := scanner.Scanner{}
	s.Init(strings.NewReader(code))
	s.Filename = "code"
	s.Mode |= scanner.ScanComments
	s.Mode &^= scanner.SkipComments

	result := ""
	lastOffset := 0
	firstHighlight := true
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		segment := code[lastOffset:s.Pos().Offset]

		var styling lipgloss.Style
		switch tok {
		case scanner.Ident:
			switch s.TokenText() {
			case "do", "def", "fn", "let", "if", "cond", "try", "catch", "finally",
				"'", "`", "quote", "quasiquote", "unquote", "splice-unquote", "~", "~@",
				"nil", "true", "false", "macroexpand", "defmacro", "defn":
				styling = SpecialFormSymbol
			case "#{":
				styling = CurlyBrackets
			default:
				styling = Symbol
			}
		case scanner.Int:
			styling = Int
		case scanner.String:
			styling = String
		case scanner.RawString:
			styling = String
		case scanner.Float:
			styling = Float
		case scanner.Comment:
			styling = Comment
		case scanner.Keyword:
			styling = Keyword
		case '(', ')':
			styling = Brackets
		case '{', '}':
			styling = CurlyBrackets
		case '[', ']':
			styling = SquareBrackets
		}
		if in(s.Pos(), start, end) {
			stylingMarked := styling.Background(lipgloss.Color("#666666")).Bold(true)
			if firstHighlight {
				var blanks string
				blanks, segment = trimBlanks(segment)
				result += styling.Render(blanks)
				result += stylingMarked.Render(segment)
				firstHighlight = false
			} else {
				result += stylingMarked.Render(segment)
			}
		} else {
			result += styling.Render(segment)
		}
		lastOffset = s.Pos().Offset
	}
	return result
}

func trimBlanks(text string) (string, string) {
	blanks := ""
	for len(text) > 0 && (text[0] == ' ' || text[0] == '\t' || text[0] == '\n' || text[0] == '\r') {
		blanks += text[:1]
		text = text[1:]
	}
	return blanks, text
}

// in checks if the cursor is within the given range.
func in(cursor, start, end scanner.Position) bool {
	return cursor.Offset > start.Offset && cursor.Offset <= end.Offset
}
