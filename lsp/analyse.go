package lsp

import (
	"regexp"
	"strings"
	"unicode/utf8"

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
	doc     string          // Clojure-style docstring, if present
	namePos *types.Position // position of the name symbol
	formPos *types.Position // position of the whole form
}

// symbolRef is one occurrence of a symbol in call-head position.
type symbolRef struct {
	name string
	pos  *types.Position
}

// requireRef is one `(require "module" [:as "a"] [:refer ["n" …]])`
// occurrence.
type requireRef struct {
	module   string
	alias    string          // :as alias; empty means the module name
	refers   []string        // :refer names imported unqualified
	referAll bool            // :refer :all imports every name unqualified
	headPos  *types.Position // position of the `require` symbol
}

// preambleDef is one leading `;; $NAME <expr>` placeholder default.
type preambleDef struct {
	name string // including the $ prefix
	expr string // the raw default expression as written
	line int    // zero-based line of the preamble entry
}

// scopeBinding is one lexical binding (a fn/defn parameter, a let/loop
// binding, or a catch variable) and the source range over which it is
// visible. The range is the whole enclosing binding form — a safe
// over-approximation of the body the binding governs, which is all that
// scope-aware reference resolution needs.
type scopeBinding struct {
	name  string
	scope Range
}

// analysis is the result of parsing one document.
type analysis struct {
	diagnostics []Diagnostic
	defs        []definition
	forms       []types.MalType // top-level forms (empty on parse error)

	calls     []symbolRef     // symbols used as the head of a call form
	bound     map[string]bool // every name bound anywhere in the document
	locals    []scopeBinding  // lexical bindings with their visibility range
	requires  []requireRef    // require'd module names (string literals)
	preambles []preambleDef   // in-file placeholder defaults
}

// analyseDocument parses content with the interpreter's reader and
// extracts diagnostics and top-level definitions.
//
// The reader parses a single form, so the document is wrapped in
// `(do\n…\n)`; the extra leading line shifts every row by one, which is
// undone by walking the AST once. This reuses the real reader — the
// same positions, the same errors the interpreter itself would report.
// emptyPlaceholders lets the reader accept `$NAME` preamble
// placeholders in analysed documents: their values are only known at
// run time (--preamble flags or Go embedding), so the editor treats
// them as nil rather than flagging every placeholder-bearing file as
// a parse error.
var emptyPlaceholders = &types.HashMap{Val: map[string]types.MalType{}}

// preambleLineRE matches one in-file placeholder default line.
var preambleLineRE = regexp.MustCompile(`^;; (\$[-\w\d]+)\s+(.+)$`)

func analyseDocument(name, content string) *analysis {
	a := &analysis{bound: map[string]bool{}}
	for i, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, ";; $") {
			break
		}
		if m := preambleLineRE.FindStringSubmatch(line); m != nil && m[1] != "$MODULE" {
			a.preambles = append(a.preambles, preambleDef{name: m[1], expr: m[2], line: i})
		}
	}
	wrapped := "(do\n" + content + "\n)"
	ast, err := reader.Read_str(wrapped, types.NewCursorFile(name), emptyPlaceholders)
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
		a.scan(form)
	}
	return a
}

// specialForms are evaluated by EVAL itself: they are neither in the
// environment nor definable, so call-head checking must skip them.
var specialForms = map[string]bool{
	"def": true, "let": true, "quote": true, "quasiquote": true,
	"quasiquoteexpand": true, "defmacro": true, "macroexpand": true,
	"try": true, "catch": true, "finally": true, "do": true, "if": true,
	"fn": true, "unquote": true, "splice-unquote": true, "context": true,
	"loop": true, "recur": true,
}

// tail returns vals[n:], or an empty slice when n is past the end.
// Documents under edit routinely contain incomplete forms (a bare
// `(fn`, `(let`, …), so slicing must never assume a minimum length.
func tail(vals []types.MalType, n int) []types.MalType {
	if n >= len(vals) {
		return nil
	}
	return vals[n:]
}

// scan walks a form collecting call-head symbol references and every
// name bound anywhere (def/defn/defmacro names, fn/defn parameters,
// let bindings, catch variables). Bindings are collected document-wide
// rather than per-scope: the goal is zero false positives on unknown-
// symbol checks, not precise scope resolution.
func (a *analysis) scan(form types.MalType) {
	list, ok := form.(types.List)
	if !ok {
		switch n := form.(type) {
		case types.Vector:
			for _, c := range n.Val {
				a.scan(c)
			}
		case types.HashMap:
			for _, v := range n.Val {
				a.scan(v)
			}
		}
		return
	}
	if len(list.Val) == 0 {
		return
	}
	head, ok := list.Val[0].(types.Symbol)
	if !ok {
		// e.g. ((fn [x] x) 1): scan every element
		for _, c := range list.Val {
			a.scan(c)
		}
		return
	}
	switch head.Val {
	case "quote", "quasiquote", "quasiquoteexpand":
		// data, not code: don't analyse
		return
	case "def", "defn", "defmacro":
		if len(list.Val) >= 2 {
			if name, ok := list.Val[1].(types.Symbol); ok {
				a.bound[name.Val] = true
			}
		}
		if head.Val != "def" && len(list.Val) >= 3 {
			a.bindAll(list.Val[2])
			a.bindLocals(list.Val[2], rangeOf(list.Cursor))
		}
		for _, c := range tail(list.Val, 2) {
			a.scan(c)
		}
	case "fn":
		if len(list.Val) >= 2 {
			a.bindAll(list.Val[1])
			a.bindLocals(list.Val[1], rangeOf(list.Cursor))
		}
		for _, c := range tail(list.Val, 2) {
			a.scan(c)
		}
	case "let", "loop":
		scope := rangeOf(list.Cursor)
		if len(list.Val) >= 2 {
			var binds []types.MalType
			switch b := list.Val[1].(type) {
			case types.Vector:
				binds = b.Val
			case types.List:
				binds = b.Val
			}
			for i := 0; i+1 < len(binds); i += 2 {
				a.bindAll(binds[i])
				a.bindLocals(binds[i], scope)
				a.scan(binds[i+1])
			}
		}
		for _, c := range tail(list.Val, 2) {
			a.scan(c)
		}
	case "catch":
		if len(list.Val) >= 2 {
			a.bindAll(list.Val[1])
			a.bindLocals(list.Val[1], rangeOf(list.Cursor))
		}
		for _, c := range tail(list.Val, 2) {
			a.scan(c)
		}
	case "require":
		// `(require "module" …)` with a literal name is statically
		// resolvable: record it (with its :as / :refer options) so the
		// server can import the module's definitions. Still recorded as
		// a call (require must exist).
		if len(list.Val) >= 2 {
			if mod, ok := list.Val[1].(string); ok {
				ref := requireRef{module: mod, headPos: head.Cursor}
				opts := tail(list.Val, 2)
				for i := 0; i+1 < len(opts); i += 2 {
					key, ok := opts[i].(string)
					if !ok {
						continue
					}
					switch key {
					case "ʞas": // keyword :as
						if alias, ok := opts[i+1].(string); ok {
							ref.alias = alias
						}
					case "ʞrefer": // keyword :refer
						switch v := opts[i+1].(type) {
						case types.Vector:
							for _, e := range v.Val {
								if name, ok := e.(string); ok {
									ref.refers = append(ref.refers, name)
								}
							}
						case string:
							// :refer :all imports every definition unqualified
							if v == types.NewKeyword("all") {
								ref.referAll = true
							}
						}
					}
				}
				a.requires = append(a.requires, ref)
			}
		}
		a.calls = append(a.calls, symbolRef{name: head.Val, pos: head.Cursor})
		for _, c := range tail(list.Val, 1) {
			a.scan(c)
		}
	default:
		if !specialForms[head.Val] {
			a.calls = append(a.calls, symbolRef{name: head.Val, pos: head.Cursor})
		}
		for _, c := range tail(list.Val, 1) {
			a.scan(c)
		}
	}
}

// bindAll records every symbol inside a binding form (a parameter
// vector, possibly with `&`, or a plain symbol) as bound.
func (a *analysis) bindAll(form types.MalType) {
	switch n := form.(type) {
	case types.Symbol:
		a.bound[n.Val] = true
	case types.Vector:
		for _, c := range n.Val {
			a.bindAll(c)
		}
	case types.List:
		for _, c := range n.Val {
			a.bindAll(c)
		}
	}
}

// bindLocals records every symbol inside a binding form (a parameter
// vector, possibly with `&`, or a plain symbol) as a lexical binding
// visible over scope. It mirrors bindAll but keeps the visibility range.
func (a *analysis) bindLocals(form types.MalType, scope Range) {
	switch n := form.(type) {
	case types.Symbol:
		if n.Val != "&" {
			a.locals = append(a.locals, scopeBinding{name: n.Val, scope: scope})
		}
	case types.Vector:
		for _, c := range n.Val {
			a.bindLocals(c, scope)
		}
	case types.List:
		for _, c := range n.Val {
			a.bindLocals(c, scope)
		}
	}
}

// posLess reports whether a is strictly before b.
func posLess(a, b Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}

// rangeContains reports whether p lies within [r.Start, r.End).
func rangeContains(r Range, p Position) bool {
	return !posLess(p, r.Start) && posLess(p, r.End)
}

// rangeInside reports whether inner is fully contained in outer and is
// not the same range — i.e. a strictly nested scope.
func rangeInside(inner, outer Range) bool {
	if inner == outer {
		return false
	}
	return !posLess(inner.Start, outer.Start) && !posLess(outer.End, inner.End)
}

// resolveScope determines which binding the symbol sym refers to at pos.
// It returns the visibility range of the innermost lexical binding of sym
// enclosing pos (isLocal=true); when none encloses pos the symbol refers
// to a top-level/free binding and the whole-document range is returned
// (isLocal=false).
func (a *analysis) resolveScope(sym string, pos Position, docScope Range) (scope Range, isLocal bool) {
	best := docScope
	found := false
	for _, b := range a.locals {
		if b.name != sym || !rangeContains(b.scope, pos) {
			continue
		}
		// Innermost wins: for properly nested scopes the one with the
		// latest start is the most deeply nested.
		if !found || posLess(best.Start, b.scope.Start) {
			best = b.scope
			found = true
		}
	}
	return best, found
}

// occurrencesInScope returns the ranges of every occurrence of sym that
// resolves to the same binding as the one under pos. It starts from the
// complete lexical occurrences (so nothing that must change is missed)
// and drops those inside a nested rebinding of the same name (shadowing).
func (a *analysis) occurrencesInScope(content, sym string, pos Position) []Range {
	docScope := wholeContentRange(content)
	target, _ := a.resolveScope(sym, pos, docScope)

	// Nested same-name bindings whose scope shadows part of the target.
	var shadows []Range
	for _, b := range a.locals {
		if b.name == sym && rangeInside(b.scope, target) {
			shadows = append(shadows, b.scope)
		}
	}

	var out []Range
	for _, occ := range symbolOccurrences(content, sym) {
		if !rangeContains(target, occ.Start) {
			continue
		}
		shadowed := false
		for _, s := range shadows {
			if rangeContains(s, occ.Start) {
				shadowed = true
				break
			}
		}
		if !shadowed {
			out = append(out, occ)
		}
	}
	return out
}

// wholeContentRange spans the entire document.
func wholeContentRange(content string) Range {
	lines := strings.Count(content, "\n")
	return Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: lines + 1, Character: 0},
	}
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
	if head.Val != "def" {
		// (defn name [params] …) or, Clojure-style,
		// (defn name "docstring" [params] …): a leading string before
		// the parameter vector is the docstring.
		rest := list.Val[2:]
		if len(rest) >= 2 {
			if s, ok := rest[0].(string); ok {
				if _, isVec := rest[1].(types.Vector); isVec {
					d.doc = s
					rest = rest[1:]
				}
			}
		}
		if len(rest) >= 1 {
			if params, ok := rest[0].(types.Vector); ok {
				d.params = printer.Pr_str(params, true)
			}
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

// symbolRange converts a symbol token position to an LSP range. The
// reader's scanner records a token's position AFTER consuming it, so
// BeginCol points just past the token's last character; the start
// column is recovered by subtracting the symbol's length.
func symbolRange(p *types.Position, name string) Range {
	if p == nil {
		return Range{}
	}
	line := p.BeginRow - 1
	if line < 0 {
		line = 0
	}
	endChar := p.BeginCol - 1
	if endChar < 0 {
		endChar = 0
	}
	startChar := endChar - len(name)
	if startChar < 0 {
		startChar = 0
	}
	return Range{
		Start: Position{Line: line, Character: startChar},
		End:   Position{Line: line, Character: endChar},
	}
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

// symbolRangeAt returns the LSP range of the symbol-shaped word around
// the given zero-based line/character, using the same boundaries as
// symbolAt. Returns a zero Range when the position does not touch a
// symbol.
func symbolRangeAt(content string, line, char int) Range {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return Range{}
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
		return Range{}
	}
	return Range{
		Start: Position{Line: line, Character: start},
		End:   Position{Line: line, Character: end},
	}
}

// symbolBreakRune is the rune-level counterpart of symbolBreak. Every
// break character is ASCII, so a non-ASCII rune is never a break.
func symbolBreakRune(r rune) bool {
	return r < utf8.RuneSelf && symbolBreak(byte(r))
}

// validSymbolName reports whether name is a single lisp symbol token: it
// must be non-empty and contain no character that would break it into
// pieces. It guards a rename's new name so a rename cannot inject
// whitespace or delimiters.
func validSymbolName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if symbolBreakRune(r) || r == '¬' {
			return false
		}
	}
	return true
}

// symbolOccurrences returns the ranges of every whole-token occurrence of
// sym in content, skipping string literals ("…" and multi-line ¬…¬) and
// line comments so a rename never rewrites data or prose. Columns are
// byte offsets within the line, matching symbolRangeAt and the rest of
// the server. Symbol tokens never span a newline, so each range is on a
// single line.
func symbolOccurrences(content, sym string) []Range {
	var out []Range
	line, col := 0, 0
	i := 0
	for i < len(content) {
		r, w := utf8.DecodeRuneInString(content[i:])
		switch {
		case r == '\n':
			line++
			col = 0
			i += w
		case r == ';':
			// Line comment: skip to (but not past) the newline.
			for i < len(content) && content[i] != '\n' {
				i++
				col++
			}
		case r == '"':
			// Single-line string with backslash escapes.
			i += w
			col += w
			for i < len(content) {
				cr, cw := utf8.DecodeRuneInString(content[i:])
				if cr == '\n' {
					break
				}
				if cr == '\\' && i+cw < len(content) {
					_, nw := utf8.DecodeRuneInString(content[i+cw:])
					i += cw + nw
					col += cw + nw
					continue
				}
				i += cw
				col += cw
				if cr == '"' {
					break
				}
			}
		case r == '¬':
			// Multi-line string; ¬¬ is an escaped ¬.
			i += w
			col += w
			for i < len(content) {
				cr, cw := utf8.DecodeRuneInString(content[i:])
				if cr == '¬' {
					if i+cw < len(content) {
						if nr, nw := utf8.DecodeRuneInString(content[i+cw:]); nr == '¬' {
							i += cw + nw
							col += cw + nw
							continue
						}
					}
					i += cw
					col += cw
					break
				}
				if cr == '\n' {
					line++
					col = 0
					i += cw
					continue
				}
				i += cw
				col += cw
			}
		case symbolBreakRune(r):
			i += w
			col += w
		default:
			// Start of a symbol token.
			startCol := col
			startI := i
			for i < len(content) {
				tr, tw := utf8.DecodeRuneInString(content[i:])
				if tr == '¬' || symbolBreakRune(tr) {
					break
				}
				i += tw
				col += tw
			}
			if content[startI:i] == sym {
				out = append(out, Range{
					Start: Position{Line: line, Character: startCol},
					End:   Position{Line: line, Character: col},
				})
			}
		}
	}
	return out
}

// offsetOf converts a zero-based line/character to a byte offset into
// content, clamped to the document.
func offsetOf(content string, line, char int) int {
	off := 0
	cur := 0
	for cur < line {
		nl := strings.IndexByte(content[off:], '\n')
		if nl < 0 {
			return len(content)
		}
		off += nl + 1
		cur++
	}
	off += char
	if off > len(content) {
		off = len(content)
	}
	return off
}

// enclosingCall finds the call form surrounding the byte offset: the
// head symbol of the innermost unmatched `(` and which argument index
// the offset falls on. It returns ok=false when the cursor is at the
// top level or the innermost open bracket is a vector/map (`[`/`{`),
// where no signature applies. Strings ("…" and ¬…¬) and line comments
// are skipped so their contents do not affect paren/argument counting.
func enclosingCall(content string, offset int) (head string, activeParam int, ok bool) {
	if offset > len(content) {
		offset = len(content)
	}
	type frame struct {
		bracket byte // '(' '[' '{'
		pos     int  // index just after the bracket
	}
	var stack []frame
	i := 0
	for i < offset {
		c := content[i]
		switch {
		case c == ';':
			// line comment to end of line
			if nl := strings.IndexByte(content[i:], '\n'); nl < 0 {
				i = offset
			} else {
				i += nl + 1
			}
			continue
		case c == '"' || c == '¬':
			term := c
			i++
			for i < offset {
				if content[i] == '\\' && term == '"' {
					i += 2
					continue
				}
				if content[i] == term {
					i++
					break
				}
				i++
			}
			continue
		case c == '(' || c == '[' || c == '{':
			stack = append(stack, frame{bracket: c, pos: i + 1})
		case c == ')' || c == ']' || c == '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
		i++
	}
	if len(stack) == 0 || stack[len(stack)-1].bracket != '(' {
		return "", 0, false
	}
	open := stack[len(stack)-1].pos

	// Head symbol: first token after the open paren.
	j := open
	for j < offset && isSpaceByte(content[j]) {
		j++
	}
	hstart := j
	for j < offset && !symbolBreak(content[j]) {
		j++
	}
	head = content[hstart:j]
	if head == "" {
		return "", 0, false
	}

	// Active parameter: count argument boundaries between the head and
	// the cursor, ignoring nested brackets and strings.
	argIndex := -1
	inArg := false
	depth := 0
	for j < offset {
		c := content[j]
		switch {
		case c == ';' && depth == 0:
			if nl := strings.IndexByte(content[j:], '\n'); nl < 0 {
				j = offset
			} else {
				j += nl + 1
			}
			continue
		case c == '"' || c == '¬':
			if depth == 0 && !inArg {
				argIndex++
				inArg = true
			}
			term := c
			j++
			for j < offset {
				if content[j] == '\\' && term == '"' {
					j += 2
					continue
				}
				if content[j] == term {
					j++
					break
				}
				j++
			}
			continue
		case c == '(' || c == '[' || c == '{':
			if depth == 0 && !inArg {
				argIndex++
				inArg = true
			}
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth > 0 {
				depth--
			}
		case isSpaceByte(c) || c == ',':
			if depth == 0 {
				inArg = false
			}
		default:
			if depth == 0 && !inArg {
				argIndex++
				inArg = true
			}
		}
		j++
	}
	if inArg {
		activeParam = argIndex
	} else {
		activeParam = argIndex + 1
	}
	if activeParam < 0 {
		activeParam = 0
	}
	return head, activeParam, true
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// splitParams turns a printed parameter vector (`[a b & rest]`) into
// individual parameter labels. The variadic marker `&` is dropped so
// the trailing rest parameter maps to a single label (arguments beyond
// the fixed ones then land on it).
func splitParams(params string) []string {
	params = strings.TrimSpace(params)
	params = strings.TrimPrefix(params, "[")
	params = strings.TrimSuffix(params, "]")
	var out []string
	for _, f := range strings.Fields(params) {
		if f != "&" {
			out = append(out, f)
		}
	}
	return out
}
