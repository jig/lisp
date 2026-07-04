package format

import "fmt"

type nodeKind int

const (
	nList nodeKind = iota // a delimited collection: ( ) [ ] { } #{ } « »
	nAtom
	nComment
)

type node struct {
	kind        nodeKind
	open, close string // for nList
	text        string // for nAtom and nComment
	prefix      string // reader-macro(s) hugging this node, e.g. "'", "~@", "^"
	children    []*node
	nlBefore    int
}

func closeFor(open string) string {
	switch open {
	case "(":
		return ")"
	case "[":
		return "]"
	case "{", "#{":
		return "}"
	case "«":
		return "»"
	}
	return ""
}

// parse turns a token stream into a tree. The root is a synthetic list
// with no delimiters whose children are the top-level forms and comments.
func parse(toks []token) (*node, error) {
	root := &node{kind: nList}
	stack := []*node{root}

	var pfx string
	var pfxNL int
	havePfx := false

	// take consumes a pending reader-macro prefix (if any) and returns the
	// prefix text plus the newline count that should govern layout.
	take := func(nl int) (string, int) {
		if havePfx {
			havePfx = false
			p := pfx
			pfx = ""
			return p, pfxNL
		}
		return "", nl
	}

	for _, t := range toks {
		cur := stack[len(stack)-1]
		switch t.kind {
		case tPrefix:
			if havePfx {
				pfx += t.text
			} else {
				havePfx, pfx, pfxNL = true, t.text, t.nlBefore
			}
		case tComment:
			if havePfx {
				return nil, fmt.Errorf("reader macro %q not followed by a form", pfx)
			}
			cur.children = append(cur.children, &node{kind: nComment, text: t.text, nlBefore: t.nlBefore})
		case tAtom:
			p, nl := take(t.nlBefore)
			cur.children = append(cur.children, &node{kind: nAtom, text: t.text, prefix: p, nlBefore: nl})
		case tOpen:
			p, nl := take(t.nlBefore)
			nn := &node{kind: nList, open: t.text, close: closeFor(t.text), prefix: p, nlBefore: nl}
			cur.children = append(cur.children, nn)
			stack = append(stack, nn)
		case tClose:
			if havePfx {
				return nil, fmt.Errorf("reader macro %q not followed by a form", pfx)
			}
			if len(stack) == 1 {
				return nil, fmt.Errorf("unexpected %q", t.text)
			}
			if cur.close != t.text {
				return nil, fmt.Errorf("mismatched delimiter: expected %q, got %q", cur.close, t.text)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if havePfx {
		return nil, fmt.Errorf("reader macro %q not followed by a form", pfx)
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("unclosed %q", stack[len(stack)-1].open)
	}
	return root, nil
}
