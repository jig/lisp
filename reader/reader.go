package reader

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/jig/scanner"

	"github.com/jig/lisp/lisperror"
	. "github.com/jig/lisp/types"
)

type Reader interface {
	next() *Token
	peek() *Token
}

type tokenReader struct {
	tokens   []Token
	position int
}

func (tr *tokenReader) next() *Token {
	if tr.position >= len(tr.tokens) {
		return nil
	}
	token := tr.tokens[tr.position]
	tr.position = tr.position + 1
	return &token
}

func (tr *tokenReader) peek() *Token {
	if tr.position >= len(tr.tokens) {
		return nil
	}
	return &tr.tokens[tr.position]
}

func tokenize(sourceCode string, cursor *Position) ([]Token, error) {
	result := make([]Token, 0, 1)

	var s scanner.Scanner
	s.Init(strings.NewReader(sourceCode))
	// Commas are whitespace, as in Clojure and kanaka/mal (the formatter's
	// lexer already treats them so): (1 2, 3) reads as (1 2 3).
	s.Whitespace |= 1 << ','
	if cursor.Module != nil {
		s.Filename = *cursor.Module
	}
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if s.ErrorCount != 0 {
			return nil, lisperror.NewLispError(fmt.Errorf("invalid token %s", s.TokenText()), &Position{
				Module:   cursor.Module,
				BeginRow: s.Pos().Line,
				BeginCol: s.Pos().Column - 1,
				Row:      s.Pos().Line,
				Col:      s.Pos().Column - 1,
			})
		}
		tokenString := s.TokenText()
		result = append(result, Token{
			Value: tokenString,
			Type:  tok,
			Cursor: Position{
				Module:   cursor.Module,
				BeginRow: s.Pos().Line,
				BeginCol: s.Pos().Column,
				Row:      s.Pos().Line,
				Col:      s.Pos().Column + s.Pos().Offset,
			},
		})
	}
	return result, nil
}

// isRadixLiteral reports whether an Int token is a radix-prefixed literal
// (0x…, 0o…, 0b…, either case), optionally preceded by a sign. Bare zero
// and leading-zero octal (042, rejected later) are not radix literals.
func isRadixLiteral(tok string) bool {
	s := tok
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		s = s[1:]
	}
	if len(s) < 2 || s[0] != '0' {
		return false
	}
	switch s[1] {
	case 'x', 'X', 'o', 'O', 'b', 'B':
		return true
	}
	return false
}

func read_atom(rdr *tokenReader) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, lisperror.NewLispError(errors.New("read_atom underflow"), tokenStruct.GetPosition())
	}
	token := &tokenStruct.Value
	switch tokenStruct.Type {
	case scanner.Int:
		// Radix-prefixed literals (0x…, 0o…, 0b…, optionally signed)
		// denote arbitrary-precision integers — serial numbers, hashes,
		// masks — and read as *big.Int, printed back as (-)0x… so they
		// round-trip. Decimal literals remain machine ints; legacy
		// leading-zero octal (042) is error-prone and rejected.
		if isRadixLiteral(*token) {
			b, ok := new(big.Int).SetString(*token, 0)
			if !ok {
				return nil, lisperror.NewLispError(errors.New("integer parse error"), tokenStruct.GetPosition())
			}
			return b, nil
		}
		if s := strings.TrimLeft(*token, "+-"); len(s) > 1 && s[0] == '0' {
			return nil, lisperror.NewLispError(errors.New("leading-zero octal literal not supported (use 0o prefix)"), tokenStruct.GetPosition())
		}
		i, err := strconv.ParseInt(*token, 0, 0)
		if err != nil {
			return nil, lisperror.NewLispError(errors.New("integer parse error"), tokenStruct.GetPosition())
		}
		return int(i), nil
	case scanner.String:
		str := (*token)[1 : len(*token)-1]
		return strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(str, `\\`, "\u029e", -1),
					`\"`, `"`, -1),
				`\n`, "\n", -1),
			"\u029e", "\\", -1), nil
	case scanner.RawString:
		if *token == "¬" {
			return nil, lisperror.NewLispError(errors.New("expected '¬', got EOF"), tokenStruct.GetPosition())
		}
		str := (*token)[2 : len(*token)-2]
		return strings.Replace(str, `¬¬`, `¬`, -1), nil
	case scanner.Keyword:
		return NewKeyword((*token)[1:len(*token)]), nil
	case scanner.Float:
		f, err := strconv.ParseFloat(*token, 32)
		if err != nil {
			return nil, lisperror.NewLispError(errors.New("float parse error"), tokenStruct.GetPosition())
		}
		return float32(f), nil
	case scanner.Ident:
		switch *token {
		case "nil":
			return nil, nil
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		fallthrough
	default:
		return Symbol{
			Val:    *token,
			Cursor: tokenStruct.GetPosition(),
		}, nil
	}
}

func read_list(rdr *tokenReader, start string, end string, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, lisperror.NewLispError(errors.New("read_list underflow"), &tokenStruct)
	}
	cursor := tokenStruct.Cursor.Copy()
	token := &tokenStruct.Value
	if *token != start {
		return nil, lisperror.NewLispError(errors.New("expected '"+start+"'"), &tokenStruct)
	}
	lastKnown := tokenStruct

	ast_list := []MalType{}
	tokenStruct = rdr.peek()
	for ; true; tokenStruct = rdr.peek() {
		if tokenStruct == nil {
			return nil, lisperror.NewLispError(errors.New("expected '"+end+"', got EOF"), lastKnown)
		}
		lastKnown = tokenStruct
		token = &tokenStruct.Value
		if *token == end {
			break
		}
		f, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		ast_list = append(ast_list, f)
	}
	rdr.next()
	return List{Val: ast_list, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
}

func read_external(rdr *tokenReader, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	lst, e := read_list(rdr, "«", "»", placeholderValues, ns)
	if e != nil {
		return nil, e
	}
	args := lst.(List).Val
	symbol := Symbol{Val: "new-" + args[0].(Symbol).Val}
	constructor, err := ns.Get(symbol)
	if err != nil {
		return nil, err
	}

	fnConstructor, ok := constructor.(Func)
	if !ok {
		return nil, lisperror.NewLispError(fmt.Errorf("attempt to call non-function (was of type %T)", constructor), lst.(List).Cursor)
	}
	typedValue, err := fnConstructor.Fn(context.Background(), args[1:])
	if err != nil {
		return nil, err
	}
	return typedValue, nil
}

func read_vector(rdr *tokenReader, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	lst, e := read_list(rdr, "[", "]", placeholderValues, ns)
	if e != nil {
		return nil, e
	}
	vec := Vector{Val: lst.(List).Val, Cursor: lst.(List).Cursor}
	return vec, nil
}

func read_hash_map(rdr *tokenReader, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	mal_lst, e := read_list(rdr, "{", "}", placeholderValues, ns)
	if e != nil {
		return nil, e
	}
	var cursor *Position
	if lst, ok := mal_lst.(List); ok {
		cursor = lst.Cursor
	}
	return NewHashMap(cursor, mal_lst)
}

func read_set(rdr *tokenReader, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	mal_lst, e := read_list(rdr, "#{", "}", placeholderValues, ns)
	if e != nil {
		return nil, e
	}
	return NewSet(mal_lst)
}

func read_placeholder(rdr *tokenReader, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, lisperror.NewLispError(errors.New("read_placeholder underflow"), &tokenStruct)
	}
	// Reading placeholder-bearing source without providing values used
	// to dereference a nil map (a runtime panic surfacing as "invalid
	// memory address" through read-string). Report it as a proper
	// parse error instead.
	if placeholderValues == nil {
		return nil, lisperror.NewLispError(fmt.Errorf("placeholder %s used but no placeholder values provided", tokenStruct.Value), tokenStruct.GetPosition())
	}
	return placeholderValues.Val[tokenStruct.Value], nil
}

func read_form(rdr *tokenReader, placeholderValues *HashMap, ns EnvType) (MalType, error) {
	tokenStruct := rdr.peek()
	if tokenStruct == nil {
		return nil, lisperror.NewLispError(errors.New("read_form underflow"), &tokenStruct)
	}
	cursor := tokenStruct.Cursor.Copy()
	switch tokenStruct.Value {
	case `'`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "quote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case "`":
		rdr.next()
		form, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "quasiquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `~`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "unquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `~@`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "splice-unquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `^`:
		rdr.next()
		meta, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		form, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "with-meta", Cursor: &tokenStruct.Cursor}, form, meta}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `@`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues, ns)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "deref", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil

	// list
	case ")":
		return nil, lisperror.NewLispError(errors.New("unexpected ')'"), tokenStruct)
	case "(":
		return read_list(rdr, "(", ")", placeholderValues, ns)

	// vector
	case "]":
		return nil, lisperror.NewLispError(errors.New("unexpected ']'"), tokenStruct)
	case "[":
		return read_vector(rdr, placeholderValues, ns)

	// hash-map
	case "}":
		return nil, lisperror.NewLispError(errors.New("unexpected '}'"), tokenStruct)
	case "{":
		return read_hash_map(rdr, placeholderValues, ns)
	case "#{":
		return read_set(rdr, placeholderValues, ns)
	case "«":
		return read_external(rdr, placeholderValues, ns)
	default:
		if len(tokenStruct.Value) > 0 && tokenStruct.Value[0] == '$' {
			return read_placeholder(rdr, placeholderValues, ns)
		}
		return read_atom(rdr)
	}
}

// ";; $MODULE ../../examples/fibonacci.lisp\n(do (do\n    (def fib\n
var moduleNamePrefixRE = regexp.MustCompile(`^;; [$]MODULE (.+)`)

// Read_str reads Lisp source code and generates
// cursor and environment might be passed nil and READ will provide correct values for you.
// It is recommended though that cursor is initialised with a source code file identifier to
// provide better positioning information in case of encountering an execution error.
//
// EnvType is required in case you expect to parse Go constructors
func Read_str(str string, cursor *Position, placeholderValues *HashMap, ns ...EnvType) (MalType, error) {
	if cursor == nil {
		cursor = NewAnonymousCursorHere(1, 1)
	}
	// `;; $MODULE <path>\n` at the head of the source claims a module
	// identity for the rest of the input. The line itself is consumed
	// before the user-relevant code begins, so every emitted cursor row
	// has to be shifted by -1 to match the path's real on-disk contents.
	rowShift := 0
	if cursor.Module == nil {
		matches := moduleNamePrefixRE.FindStringSubmatch(str)
		if matches != nil {
			cursor = NewCursorFile(matches[1])
			rowShift = 1
		}
	}
	tokens, err := tokenize(str, cursor)
	if err != nil {
		return nil, err
	}
	if rowShift != 0 {
		for i := range tokens {
			tokens[i].Cursor.BeginRow -= rowShift
			if tokens[i].Cursor.Row > 0 {
				tokens[i].Cursor.Row -= rowShift
			}
		}
	}
	if len(tokens) == 0 {
		return nil, errors.New("<empty line>")
	}

	tokenReader := tokenReader{
		tokens:   tokens,
		position: 0,
	}

	var nsv EnvType
	if len(ns) == 0 {
		nsv = nil
	} else {
		nsv = ns[0]
	}
	res, err := read_form(&tokenReader, placeholderValues, nsv)
	if err != nil {
		return nil, err
	}
	if tokenReader.position != len(tokenReader.tokens) {
		return nil, lisperror.NewLispError(errors.New("not all tokens where parsed"), tokenReader.tokens[tokenReader.position-1])
	}
	return res, nil
}
