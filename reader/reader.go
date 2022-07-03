package reader

import (
	"errors"
	"fmt"
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
	tokenizerRE  = regexp.MustCompile(`(?:\n|[ \r\t,]*)(~@|#{|[$][0-9]|[\[\]{}()'` + "`" + `~^@]|"(?:\\.|[^\\"])*"?|¬[^¬]*(?:(?:¬¬)[^¬]*)*¬?|;.*|[^\s\[\]{}('"` + "`" + `,;)]*)`)
	integerRE    = regexp.MustCompile(`^-?[0-9]+$`)
	stringRE     = regexp.MustCompile(`^"(?:\\.|[^\\"])*"$`)
	jsonStringRE = regexp.MustCompile(`^¬[^¬]*(?:(?:¬¬)[^¬]*)*¬$`)
)

func tokenize(str string, cursor *Position) []Token {
	if cursor == nil {
		cursor = &Position{Row: 1, Col: 1}
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
		return nil, RuntimeError{ErrorVal: errors.New("read_atom underflow"), Cursor: &tokenStruct.Cursor}
	}
	token := &tokenStruct.Value
	if match := integerRE.MatchString(*token); match {
		var i int
		var e error
		if i, e = strconv.Atoi(*token); e != nil {
			return nil, RuntimeError{ErrorVal: errors.New("number parse error"), Cursor: &tokenStruct.Cursor}
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
		return nil, RuntimeError{ErrorVal: errors.New("expected '\"', got EOF"), Cursor: &tokenStruct.Cursor}
	} else if match := jsonStringRE.MatchString(*token); match {
		str := (*token)[2 : len(*token)-2]
		return strings.Replace(str, `¬¬`, `¬`, -1), nil
	} else if (*token)[0] == '¬' {
		return nil, RuntimeError{ErrorVal: errors.New("expected '¬', got EOF"), Cursor: &tokenStruct.Cursor}
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

func read_list(rdr Reader, start string, end string, placeholderValues []string) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, RuntimeError{ErrorVal: errors.New("read_list underflow"), Cursor: &tokenStruct.Cursor}
	}
	token := &tokenStruct.Value
	if *token != start {
		return nil, RuntimeError{ErrorVal: errors.New("expected '" + start + "'"), Cursor: &tokenStruct.Cursor}
	}
	lastKnown := tokenStruct

	ast_list := []MalType{}
	tokenStruct = rdr.peek()
	for ; true; tokenStruct = rdr.peek() {
		if tokenStruct == nil {
			return nil, RuntimeError{ErrorVal: errors.New("expected '" + end + "', got EOF"), Cursor: &lastKnown.Cursor}
		}
		lastKnown = tokenStruct
		token = &tokenStruct.Value
		if *token == end {
			break
		}
		f, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		ast_list = append(ast_list, f)
	}
	rdr.next()
	return List{Val: ast_list, Cursor: &tokenStruct.Cursor}, nil
}

func read_vector(rdr Reader, placeholderValues []string) (MalType, error) {
	lst, e := read_list(rdr, "[", "]", placeholderValues)
	if e != nil {
		return nil, e
	}
	vec := Vector{Val: lst.(List).Val, Cursor: lst.(List).Cursor}
	return vec, nil
}

func read_hash_map(rdr Reader, placeholderValues []string) (MalType, error) {
	mal_lst, e := read_list(rdr, "{", "}", placeholderValues)
	if e != nil {
		return nil, e
	}
	return NewHashMap(mal_lst)
}

func read_set(rdr Reader, placeholderValues []string) (MalType, error) {
	mal_lst, e := read_list(rdr, "#{", "}", placeholderValues)
	if e != nil {
		return nil, e
	}
	return NewSet(mal_lst)
}

func read_placeholder(rdr Reader, placeholderValues []string) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, RuntimeError{ErrorVal: errors.New("read_placeholder underflow"), Cursor: &tokenStruct.Cursor}
	}
	token := tokenStruct.Value[1:]
	if match := integerRE.MatchString(token); match {
		i, err := strconv.Atoi(token)
		if err != nil {
			return nil, RuntimeError{ErrorVal: errors.New("number parse error"), Cursor: &tokenStruct.Cursor}
		}
		if len(placeholderValues) > i {
			return placeholderValues[i], nil
		}
		return nil, fmt.Errorf("placeholder %s undefined", token)
	} else {
		return nil, RuntimeError{ErrorVal: errors.New("read_placeholder requires a number argument"), Cursor: &tokenStruct.Cursor}
	}
}

func read_form(rdr Reader, placeholderValues []string) (MalType, error) {
	tokenStruct := rdr.peek()
	if tokenStruct == nil {
		return nil, RuntimeError{ErrorVal: errors.New("read_form underflow"), Cursor: &tokenStruct.Cursor}
	}
	switch tokenStruct.Value {
	case `'`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "quote", Cursor: &tokenStruct.Cursor}, form}, Cursor: &tokenStruct.Cursor}, nil
	case "`":
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "quasiquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: &tokenStruct.Cursor}, nil
	case `~`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "unquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: &tokenStruct.Cursor}, nil
	case `~@`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "splice-unquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: &tokenStruct.Cursor}, nil
	case `^`:
		rdr.next()
		meta, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "with-meta", Cursor: &tokenStruct.Cursor}, form, meta}, Cursor: &tokenStruct.Cursor}, nil
	case `@`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "deref", Cursor: &tokenStruct.Cursor}, form}, Cursor: &tokenStruct.Cursor}, nil

	// list
	case ")":
		return nil, RuntimeError{ErrorVal: errors.New("unexpected ')'"), Cursor: &tokenStruct.Cursor}
	case "(":
		return read_list(rdr, "(", ")", placeholderValues)

	// vector
	case "]":
		return nil, RuntimeError{ErrorVal: errors.New("unexpected ']'"), Cursor: &tokenStruct.Cursor}
	case "[":
		return read_vector(rdr, placeholderValues)

	// hash-map
	case "}":
		return nil, RuntimeError{ErrorVal: errors.New("unexpected '}'"), Cursor: &tokenStruct.Cursor}
	case "{":
		return read_hash_map(rdr, placeholderValues)
	case "#{":
		return read_set(rdr, placeholderValues)
	case "$0", "$1", "$2", "$3", "$4", "$5", "$6", "$7", "$8", "$9":
		return read_placeholder(rdr, placeholderValues)
	default:
		return read_atom(rdr)
	}
}

func Read_str(str string, cursor *Position, placeholderValues []string) (MalType, error) {
	var tokens = tokenize(str, cursor)
	if len(tokens) == 0 {
		return nil, errors.New("<empty line>")
	}

	return read_form(
		&TokenReader{
			tokens:   tokens,
			position: 0,
		}, placeholderValues,
	)
}
