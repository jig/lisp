package format

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type tokKind int

const (
	tOpen tokKind = iota
	tClose
	tAtom
	tPrefix
	tComment
)

type token struct {
	kind tokKind
	text string
	// nlBefore is the number of newlines between the previous token and
	// this one: 0 means same line, 1 a new line, 2+ at least one blank
	// line in between.
	nlBefore int
}

// lex splits jig/lisp source into tokens, keeping comments and enough
// newline information to reconstruct the author's line breaks.
func lex(src string) ([]token, error) {
	var toks []token
	i, n := 0, len(src)
	nl := 0

	emit := func(k tokKind, text string) {
		toks = append(toks, token{kind: k, text: text, nlBefore: nl})
		nl = 0
	}

	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			nl++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' || c == ',':
			// commas are whitespace in Lisp
			i++
		case c == ';':
			j := i
			for j < n && src[j] != '\n' {
				j++
			}
			emit(tComment, strings.TrimRight(src[i:j], " \t\r"))
			i = j
		case c == '(' || c == '[' || c == '{':
			emit(tOpen, string(c))
			i++
		case c == ')' || c == ']' || c == '}':
			emit(tClose, string(c))
			i++
		case c == '\'' || c == '`' || c == '@' || c == '^':
			emit(tPrefix, string(c))
			i++
		case c == '~':
			if i+1 < n && src[i+1] == '@' {
				emit(tPrefix, "~@")
				i += 2
			} else {
				emit(tPrefix, "~")
				i++
			}
		case c == '"':
			j, closed := i+1, false
			for j < n {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == '"' {
					j++
					closed = true
					break
				}
				j++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated string")
			}
			emit(tAtom, src[i:j])
			i = j
		case strings.HasPrefix(src[i:], "#{"):
			emit(tOpen, "#{")
			i += 2
		case strings.HasPrefix(src[i:], "«"):
			emit(tOpen, "«")
			i += len("«")
		case strings.HasPrefix(src[i:], "»"):
			emit(tClose, "»")
			i += len("»")
		case strings.HasPrefix(src[i:], "¬"):
			// Raw string terminated by a lone ¬; ¬¬ is an escaped literal ¬.
			j, closed := i+len("¬"), false
			for j < n {
				if strings.HasPrefix(src[j:], "¬¬") {
					j += 2 * len("¬")
					continue
				}
				if strings.HasPrefix(src[j:], "¬") {
					j += len("¬")
					closed = true
					break
				}
				j++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated raw string")
			}
			emit(tAtom, src[i:j])
			i = j
		default:
			// A bare atom: symbol, number, keyword or $placeholder. It runs
			// until whitespace, a delimiter, a comment or a string.
			j := i
			for j < n {
				if isAtomBoundary(src[j:]) {
					break
				}
				_, w := utf8.DecodeRuneInString(src[j:])
				j += w
			}
			emit(tAtom, src[i:j])
			i = j
		}
	}
	return toks, nil
}

// isAtomBoundary reports whether s begins with a rune that terminates a
// bare atom.
func isAtomBoundary(s string) bool {
	switch s[0] {
	case ' ', '\t', '\n', '\r', '\f', '\v', ',',
		'(', ')', '[', ']', '{', '}', ';', '"':
		return true
	}
	return strings.HasPrefix(s, "«") || strings.HasPrefix(s, "»") || strings.HasPrefix(s, "¬")
}
