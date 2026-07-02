package lsp

import (
	"strings"

	"github.com/jig/lisp/lisperror"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"
)

// definition is a top-level def/defn/defmacro found in a document.
type definition struct {
	name    string
	kind    string          // "def" | "defn" | "defmacro"
	params  string          // pr_str of the parameter vector for defn/defmacro
	namePos *types.Position // position of the name symbol
	formPos *types.Position // position of the whole form
}

// analysis is the result of parsing one document.
type analysis struct {
	diagnostics []Diagnostic
	defs        []definition
	forms       []types.MalType // top-level forms (empty on parse error)
}

// analyseDocument parses content with the interpreter's reader and
// extracts diagnostics and top-level definitions.
//
// The reader parses a single form, so the document is wrapped in
// `(do\n…\n)`; the extra leading line shifts every row by one, which is
// undone by walking the AST once. This reuses the real reader — the
// same positions, the same errors the interpreter itself would report.
func analyseDocument(name, content string) *analysis {
	a := &analysis{}
	wrapped := "(do\n" + content + "\n)"
	ast, err := reader.Read_str(wrapped, types.NewCursorFile(name), nil)
	if err != nil {
		a.diagnostics = append(a.diagnostics, diagnosticFromError(err))
		return a
	}
	shiftRows(ast, -1, map[*types.Position]bool{})
	wrapper, ok := ast.(types.List)
	if !ok || len(wrapper.Val) == 0 {
		return a
	}
	a.forms = wrapper.Val[1:] // skip the `do` head symbol
	for _, form := range a.forms {
		if d, ok := definitionOf(form); ok {
			a.defs = append(a.defs, d)
		}
	}
	return a
}

// diagnosticFromError converts a reader error into an LSP diagnostic.
// Rows are shifted by -1 to undo the `(do\n` wrapper line; LSP positions
// are zero-based while the reader's are one-based.
func diagnosticFromError(err error) Diagnostic {
	d := Diagnostic{
		Severity: severityError,
		Source:   "lisp",
		Message:  err.Error(),
	}
	var pos *types.Position
	if lispErr, ok := err.(lisperror.LispError); ok {
		pos = lispErr.Position()
		// The wrapped LispError message embeds "<module>:<row>:", with the
		// row offset by the `(do\n` wrapper line. The diagnostic range
		// already carries the (corrected) position, so surface only the
		// bare underlying message.
		if inner, ok := lispErr.ErrorValue().(error); ok {
			d.Message = inner.Error()
		}
	}
	if pos == nil {
		d.Range = Range{Start: Position{0, 0}, End: Position{0, 1}}
		return d
	}
	startLine := pos.BeginRow - 2 // -1 wrapper line, -1 zero-based
	if startLine < 0 {
		startLine = 0
	}
	startChar := pos.BeginCol - 1
	if startChar < 0 {
		startChar = 0
	}
	endLine := pos.Row - 2
	endChar := pos.Col
	if endLine < startLine || (endLine == startLine && endChar <= startChar) {
		endLine = startLine
		endChar = startChar + 1
	}
	d.Range = Range{
		Start: Position{Line: startLine, Character: startChar},
		End:   Position{Line: endLine, Character: endChar},
	}
	return d
}

// definitionOf recognises `(def name …)`, `(defn name [params] …)` and
// `(defmacro name …)` forms.
func definitionOf(form types.MalType) (definition, bool) {
	list, ok := form.(types.List)
	if !ok || len(list.Val) < 2 {
		return definition{}, false
	}
	head, ok := list.Val[0].(types.Symbol)
	if !ok {
		return definition{}, false
	}
	switch head.Val {
	case "def", "defn", "defmacro":
	default:
		return definition{}, false
	}
	name, ok := list.Val[1].(types.Symbol)
	if !ok {
		return definition{}, false
	}
	d := definition{
		name:    name.Val,
		kind:    head.Val,
		namePos: name.Cursor,
		formPos: list.Cursor,
	}
	if head.Val != "def" && len(list.Val) >= 3 {
		if params, ok := list.Val[2].(types.Vector); ok {
			d.params = printer.Pr_str(params, true)
		}
	}
	return d, true
}

// shiftRows walks the AST adjusting every cursor's rows by delta. seen
// guards against adjusting a shared *Position twice.
func shiftRows(ast types.MalType, delta int, seen map[*types.Position]bool) {
	switch n := ast.(type) {
	case types.List:
		shiftPos(n.Cursor, delta, seen)
		for _, c := range n.Val {
			shiftRows(c, delta, seen)
		}
	case types.Vector:
		shiftPos(n.Cursor, delta, seen)
		for _, c := range n.Val {
			shiftRows(c, delta, seen)
		}
	case types.HashMap:
		shiftPos(n.Cursor, delta, seen)
		for _, v := range n.Val {
			shiftRows(v, delta, seen)
		}
	case types.Set:
		shiftPos(n.Cursor, delta, seen)
	case types.Symbol:
		shiftPos(n.Cursor, delta, seen)
	}
}

func shiftPos(p *types.Position, delta int, seen map[*types.Position]bool) {
	if p == nil || seen[p] {
		return
	}
	seen[p] = true
	p.BeginRow += delta
	if p.Row > 0 {
		p.Row += delta
	}
}

// rangeOf converts a reader position (one-based) to an LSP range
// (zero-based).
func rangeOf(p *types.Position) Range {
	if p == nil {
		return Range{}
	}
	start := Position{Line: p.BeginRow - 1, Character: p.BeginCol - 1}
	if start.Line < 0 {
		start.Line = 0
	}
	if start.Character < 0 {
		start.Character = 0
	}
	end := Position{Line: p.Row - 1, Character: p.Col}
	if end.Line < start.Line || (end.Line == start.Line && end.Character <= start.Character) {
		end = Position{Line: start.Line, Character: start.Character + 1}
	}
	return Range{Start: start, End: end}
}

// symbolBreak reports whether b terminates a lisp symbol.
func symbolBreak(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '(', ')', '[', ']', '{', '}', '"', '\'', '`', '~', '@', '^', ';', ',':
		return true
	}
	return false
}

// symbolAt extracts the symbol-shaped word around the given zero-based
// line/character of content. Returns "" when the position does not touch
// a symbol.
func symbolAt(content string, line, char int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	l := lines[line]
	if char > len(l) {
		char = len(l)
	}
	start, end := char, char
	for start > 0 && !symbolBreak(l[start-1]) {
		start--
	}
	for end < len(l) && !symbolBreak(l[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return l[start:end]
}
