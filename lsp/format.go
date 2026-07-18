package lsp

// Support for format strings: the %verbs inside the string literal of a
// (format "..." args...) call are highlighted as semantic tokens, and a
// mismatch between the verbs and the call's arguments is reported as a
// warning, in the spirit of what gopls does for Printf-style calls.
//
// The reader keeps no source position for string literals (they are
// plain Go strings in the AST), so the literal is located by scanning
// the document text right after the `format` head symbol, whose
// position is known.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jig/lisp/types"
)

// docIndex converts between absolute byte offsets in a document and
// LSP line/character positions.
type docIndex struct {
	content   string
	lineStart []int
}

func newDocIndex(content string) *docIndex {
	starts := []int{0}
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &docIndex{content: content, lineStart: starts}
}

func (d *docIndex) offset(p Position) int {
	if p.Line < 0 || p.Line >= len(d.lineStart) {
		return -1
	}
	off := d.lineStart[p.Line] + p.Character
	if off > len(d.content) {
		return -1
	}
	return off
}

func (d *docIndex) pos(off int) Position {
	line := sort.Search(len(d.lineStart), func(i int) bool { return d.lineStart[i] > off }) - 1
	return Position{Line: line, Character: off - d.lineStart[line]}
}

// stringLiteral is a located format string: the raw source text between
// the delimiters and the absolute offsets of its full extent.
type stringLiteral struct {
	inner      string // source text between the delimiters (escapes not decoded)
	innerStart int    // absolute offset of inner's first byte
	start, end int    // absolute offsets of the literal including delimiters
}

// literalAfter locates the string literal that follows the head symbol
// of a (format ...) call in the source text. It only skips whitespace,
// so anything else (a variable, a comment) makes it bail out.
func literalAfter(idx *docIndex, head types.Symbol) (stringLiteral, bool) {
	off := idx.offset(symbolRange(head.Cursor, head.Val).End)
	if off < 0 {
		return stringLiteral{}, false
	}
	content := idx.content
	for off < len(content) && strings.ContainsRune(" \t\r\n,", rune(content[off])) {
		off++
	}
	switch {
	case off < len(content) && content[off] == '"':
		for j := off + 1; j < len(content); j++ {
			switch content[j] {
			case '\\':
				j++
			case '"':
				return stringLiteral{inner: content[off+1 : j], innerStart: off + 1, start: off, end: j + 1}, true
			}
		}
	case strings.HasPrefix(content[off:], "¬"):
		d := len("¬")
		if rel := strings.Index(content[off+d:], "¬"); rel >= 0 {
			return stringLiteral{inner: content[off+d : off+d+rel], innerStart: off + d, start: off, end: off + d + rel + d}, true
		}
	}
	return stringLiteral{}, false
}

// formatVerb is one % directive found in a format string.
type formatVerb struct {
	off, len int  // byte extent within the literal's inner text
	consumes int  // arguments consumed: 1 per verb plus 1 per '*'
	indexed  bool // uses an explicit [n] argument index
	literal  bool // %%: no argument
	invalid  bool // no verb letter could be parsed
}

// scanFormatVerbs finds the % directives in the raw source text of a
// format string. Escape sequences (\n, \") contain no '%', so scanning
// source text yields the same directives as the decoded value.
func scanFormatVerbs(s string) []formatVerb {
	var out []formatVerb
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		v := formatVerb{off: i, consumes: 1}
		j := i + 1
		for j < len(s) && strings.ContainsRune("#0- +'", rune(s[j])) {
			j++
		}
		if j < len(s) && s[j] == '[' {
			v.indexed = true
			for j < len(s) && s[j] != ']' {
				j++
			}
			if j < len(s) {
				j++
			}
		}
		for j < len(s) && (s[j] == '*' || (s[j] >= '0' && s[j] <= '9')) {
			if s[j] == '*' {
				v.consumes++
			}
			j++
		}
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && (s[j] == '*' || (s[j] >= '0' && s[j] <= '9')) {
				if s[j] == '*' {
					v.consumes++
				}
				j++
			}
		}
		switch {
		case j >= len(s):
			v.invalid = true
			v.consumes = 0
		case s[j] == '%':
			j++
			v.literal = j-i == 2 // "%[1]%" and friends stay invalid-ish but harmless
			v.consumes = 0
		case (s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z'):
			j++
		default:
			j++
			v.invalid = true
			v.consumes = 0
		}
		v.len = j - i
		out = append(out, v)
		i = j - 1
	}
	return out
}

// formatCall matches a (format "..." args...) call form and locates its
// literal; ok is false for empty lists, other heads or non-literal
// format arguments.
func formatCall(idx *docIndex, n types.List) (head types.Symbol, lit stringLiteral, ok bool) {
	if len(n.Val) < 2 {
		return head, lit, false
	}
	head, isSym := n.Val[0].(types.Symbol)
	if !isSym || head.Val != "format" || head.Cursor == nil {
		return head, lit, false
	}
	if _, isString := n.Val[1].(string); !isString {
		return head, lit, false
	}
	lit, ok = literalAfter(idx, head)
	return head, lit, ok
}

// formatToks emits one semantic token per % directive of a format call,
// so verbs read distinctly inside the string. Reuses the keyword token
// type: no legend (nor VS Code extension) change needed.
func (b *semBuilder) formatToks(n types.List) {
	if b.idx == nil {
		return
	}
	_, lit, ok := formatCall(b.idx, n)
	if !ok {
		return
	}
	for _, v := range scanFormatVerbs(lit.inner) {
		start := b.idx.pos(lit.innerStart + v.off)
		end := b.idx.pos(lit.innerStart + v.off + v.len)
		if start.Line != end.Line {
			continue // a directive never spans lines; guard anyway
		}
		b.toks = append(b.toks, semTok{rng: Range{Start: start, End: end}, typ: tokKeyword})
	}
}

// formatDiagnostics walks the document's forms and reports format calls
// whose argument count cannot match the format string, plus malformed
// directives. Calls with explicit [n] indexes skip the count check, and
// non-literal format strings are ignored.
func formatDiagnostics(anal *analysis, content string) []Diagnostic {
	idx := newDocIndex(content)
	var diags []Diagnostic
	var walk func(form types.MalType)
	walk = func(form types.MalType) {
		switch n := form.(type) {
		case types.Vector:
			for _, c := range n.Val {
				walk(c)
			}
		case types.HashMap:
			for _, v := range n.Val {
				walk(v)
			}
		case types.List:
			if len(n.Val) == 0 {
				return
			}
			if head, ok := n.Val[0].(types.Symbol); ok {
				switch head.Val {
				case "quote", "quasiquote", "quasiquoteexpand":
					return // data, not code
				}
			}
			diags = append(diags, formatCallDiagnostics(idx, n)...)
			for _, c := range n.Val {
				walk(c)
			}
		}
	}
	for _, f := range anal.forms {
		walk(f)
	}
	return diags
}

func formatCallDiagnostics(idx *docIndex, n types.List) []Diagnostic {
	_, lit, ok := formatCall(idx, n)
	if !ok {
		return nil
	}
	var diags []Diagnostic
	litRange := Range{Start: idx.pos(lit.start), End: idx.pos(lit.end)}
	consumes, indexed := 0, false
	for _, v := range scanFormatVerbs(lit.inner) {
		if v.invalid {
			diags = append(diags, Diagnostic{
				Range: Range{
					Start: idx.pos(lit.innerStart + v.off),
					End:   idx.pos(lit.innerStart + v.off + v.len),
				},
				Severity: severityWarning,
				Source:   "lisp",
				Message:  "invalid format directive",
			})
			continue
		}
		consumes += v.consumes
		indexed = indexed || v.indexed
	}
	if args := len(n.Val) - 2; !indexed && consumes != args {
		diags = append(diags, Diagnostic{
			Range:    litRange,
			Severity: severityWarning,
			Source:   "lisp",
			Message:  fmt.Sprintf("format string consumes %d argument(s), but %d given", consumes, args),
		})
	}
	return diags
}
