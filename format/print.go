package format

import (
	"strings"
	"unicode/utf8"
)

type printer struct {
	b      strings.Builder
	curCol int // current column (in runes) on the line being written
}

func (p *printer) write(s string) {
	p.b.WriteString(s)
	if idx := strings.LastIndexByte(s, '\n'); idx >= 0 {
		p.curCol = utf8.RuneCountInString(s[idx+1:])
	} else {
		p.curCol += utf8.RuneCountInString(s)
	}
}

// printForm writes a single node, positioned at the current column.
func (p *printer) printForm(n *node) {
	if n.prefix != "" {
		p.write(n.prefix)
	}
	switch n.kind {
	case nAtom, nComment:
		p.write(n.text)
	case nList:
		openCol := p.curCol
		p.write(n.open)
		childIndent := openCol + runeLen(n.open) // align to first element
		if n.open == "(" {
			childIndent = openCol + 2 // two-space body indent for ( ) lists
		}
		p.printChildList(n, childIndent, openCol)
	}
}

// printChildList writes a collection's children (the caller has already
// emitted the opening delimiter) followed by the closing delimiter.
// childIndent is the column for children that start a new line; openCol is
// the column of the opening delimiter, used when the closer must drop to
// its own line.
func (p *printer) printChildList(n *node, childIndent, openCol int) {
	lastComment := false
	for i, ch := range n.children {
		switch {
		case i == 0:
			// first child hugs the opening delimiter (and so drops any
			// blank line between the delimiter and the first form)
			p.printForm(ch)
		case ch.nlBefore == 0 && !lastComment:
			p.write(" ")
			p.printForm(ch)
		default:
			// a comment always ends its line, so what follows must wrap even
			// if it shared the line in the source
			if ch.nlBefore >= 2 {
				p.write("\n") // collapse a run of blank lines to one
			}
			p.write("\n")
			p.write(strings.Repeat(" ", childIndent))
			p.printForm(ch)
		}
		lastComment = ch.kind == nComment
	}
	if n.open == "" {
		return // synthetic root: no delimiters
	}
	if lastComment {
		// the closer cannot share a line with a trailing comment
		p.write("\n")
		p.write(strings.Repeat(" ", openCol))
	}
	p.write(n.close)
}

func (p *printer) result() string {
	s := strings.TrimLeft(p.b.String(), "\n")
	s = strings.TrimRight(s, " \t\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}
