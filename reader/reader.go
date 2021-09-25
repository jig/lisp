package reader

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	//"fmt"

	. "github.com/jig/lisp/types"
)

type Reader interface {
	next() *Token
	peek() *Token
}

type TokenReader struct {
	tokens   []Token
	position int
}

func (tr *TokenReader) next() *Token {
	if tr.position >= len(tr.tokens) {
		return nil
	}
	token := tr.tokens[tr.position]
	tr.position = tr.position + 1
	return &token
}

func (tr *TokenReader) peek() *Token {
	if tr.position >= len(tr.tokens) {
		return nil
	}
	return &tr.tokens[tr.position]
}

var (
	tokenizerRE  = regexp.MustCompile(`(?:\n|[ \r\t,]*)(~@|[\[\]{}()'` + "`" + `~^@]|"(?:\\.|[^\\"])*"?|¬[^¬]*(?:(?:¬¬)[^¬]*)*¬?|;.*|[^\s\[\]{}('"` + "`" + `,;)]*)`)
	integerRE    = regexp.MustCompile(`^-?[0-9]+$`)
	stringRE     = regexp.MustCompile(`^"(?:\\.|[^\\"])*"$`)
	jsonStringRE = regexp.MustCompile(`^¬[^¬]*(?:(?:¬¬)[^¬]*)*¬$`)
)

func tokenize(str string, cursor *Position) []Token {
	if cursor == nil {
		cursor = &Position{nil, 1, 1}
	}
	results := make([]Token, 0, 1)
	for _, group := range tokenizerRE.FindAllStringSubmatch(str, -1) {
		if group[0] == "" {
			continue
		}
		if group[0][0] == '\n' {
			cursor.Row++
			cursor.Col = 0
		}
		if (group[1] == "") || (group[1][0] == ';') {
			continue
		}
		results = append(results, Token{
			Value:  group[1],
			Cursor: *cursor,
		})
		cursor.Col += len(group[0])
	}
	return results
}

func read_atom(rdr Reader) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, RuntimeError{errors.New("read_atom underflow"), "", nil, &tokenStruct.Cursor}
	}
	token := &tokenStruct.Value
	if match := integerRE.MatchString(*token); match {
		var i int
		var e error
		if i, e = strconv.Atoi(*token); e != nil {
			return nil, RuntimeError{errors.New("number parse error"), "", nil, &tokenStruct.Cursor}
		}
		return i, nil
	} else if match := stringRE.MatchString(*token); match {
		str := (*token)[1 : len(*token)-1]
		return strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(str, `\\`, "\u029e", -1),
					`\"`, `"`, -1),
				`\n`, "\n", -1),
			"\u029e", "\\", -1), nil
	} else if (*token)[0] == '"' {
		return nil, RuntimeError{errors.New("expected '\"', got EOF"), "", nil, &tokenStruct.Cursor}
	} else if match := jsonStringRE.MatchString(*token); match {
		str := (*token)[2 : len(*token)-2]
		return strings.Replace(str, `¬¬`, `¬`, -1), nil
	} else if (*token)[0] == '¬' {
		return nil, RuntimeError{errors.New("expected '¬', got EOF"), "", nil, &tokenStruct.Cursor}
	} else if (*token)[0] == ':' {
		return NewKeyword((*token)[1:len(*token)])
	} else if *token == "nil" {
		return nil, nil
	} else if *token == "true" {
		return true, nil
	} else if *token == "false" {
		return false, nil
	} else {
		return Symbol{Val: *token, Cursor: &tokenStruct.Cursor}, nil
	}
}

func read_list(rdr Reader, start string, end string) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, RuntimeError{errors.New("read_list underflow"), "", nil, &tokenStruct.Cursor}
	}
	token := &tokenStruct.Value
	if *token != start {
		return nil, RuntimeError{errors.New("expected '" + start + "'"), "", nil, &tokenStruct.Cursor}
	}
	lastKnown := tokenStruct

	ast_list := []MalType{}
	tokenStruct = rdr.peek()
	for ; true; tokenStruct = rdr.peek() {
		if tokenStruct == nil {
			return nil, RuntimeError{errors.New("expected '" + end + "', got EOF"), "", nil, &lastKnown.Cursor}
		}
		lastKnown = tokenStruct
		token = &tokenStruct.Value
		if *token == end {
			break
		}
		f, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		ast_list = append(ast_list, f)
	}
	rdr.next()
	return List{ast_list, nil, &tokenStruct.Cursor}, nil
}

func read_vector(rdr Reader) (MalType, error) {
	lst, e := read_list(rdr, "[", "]")
	if e != nil {
		return nil, e
	}
	vec := Vector{lst.(List).Val, nil, lst.(List).Cursor}
	return vec, nil
}

func read_hash_map(rdr Reader) (MalType, error) {
	mal_lst, e := read_list(rdr, "{", "}")
	if e != nil {
		return nil, e
	}
	return NewHashMap(mal_lst)
}

func read_form(rdr Reader) (MalType, error) {
	tokenStruct := rdr.peek()
	if tokenStruct == nil {
		return nil, RuntimeError{errors.New("read_form underflow"), "", nil, &tokenStruct.Cursor}
	}
	switch tokenStruct.Value {
	case `'`:
		rdr.next()
		form, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		return List{[]MalType{Symbol{Val: "quote", Cursor: &tokenStruct.Cursor}, form}, nil, &tokenStruct.Cursor}, nil
	case "`":
		rdr.next()
		form, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		return List{[]MalType{Symbol{Val: "quasiquote", Cursor: &tokenStruct.Cursor}, form}, nil, &tokenStruct.Cursor}, nil
	case `~`:
		rdr.next()
		form, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		return List{[]MalType{Symbol{Val: "unquote", Cursor: &tokenStruct.Cursor}, form}, nil, &tokenStruct.Cursor}, nil
	case `~@`:
		rdr.next()
		form, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		return List{[]MalType{Symbol{Val: "splice-unquote", Cursor: &tokenStruct.Cursor}, form}, nil, &tokenStruct.Cursor}, nil
	case `^`:
		rdr.next()
		meta, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		form, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		return List{[]MalType{Symbol{Val: "with-meta", Cursor: &tokenStruct.Cursor}, form, meta}, nil, &tokenStruct.Cursor}, nil
	case `@`:
		rdr.next()
		form, e := read_form(rdr)
		if e != nil {
			return nil, e
		}
		return List{[]MalType{Symbol{Val: "deref", Cursor: &tokenStruct.Cursor}, form}, nil, &tokenStruct.Cursor}, nil

	// list
	case ")":
		return nil, RuntimeError{errors.New("unexpected ')'"), "", nil, &tokenStruct.Cursor}
	case "(":
		return read_list(rdr, "(", ")")

	// vector
	case "]":
		return nil, RuntimeError{errors.New("unexpected ']'"), "", nil, &tokenStruct.Cursor}
	case "[":
		return read_vector(rdr)

	// hash-map
	case "}":
		return nil, RuntimeError{errors.New("unexpected '}'"), "", nil, &tokenStruct.Cursor}
	case "{":
		return read_hash_map(rdr)
	default:
		return read_atom(rdr)
	}
}

func Read_str(str string, cursor *Position) (MalType, error) {
	var tokens = tokenize(str, cursor)
	if len(tokens) == 0 {
		return nil, errors.New("<empty line>")
	}

	return read_form(&TokenReader{
		tokens:   tokens,
		position: 0,
	})
}
