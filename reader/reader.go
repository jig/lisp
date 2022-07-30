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
	tokenizerRE  = regexp.MustCompile(`(?:\n|[ \r\t,]*)(~@|#{|\$[0-9A-Z]+|[\[\]{}()'` + "`" + `~^@]|"(?:\\.|[^\\"])*"?|¬[^¬]*(?:(?:¬¬)[^¬]*)*¬?|;.*|[^\s\[\]{}('"` + "`" + `,;)]*)`)
	integerRE    = regexp.MustCompile(`^-?[0-9]+$`)
	stringRE     = regexp.MustCompile(`^"(?:\\.|[^\\"])*"$`)
	jsonStringRE = regexp.MustCompile(`^¬[^¬]*(?:(?:¬¬)[^¬]*)*¬$`)
)

func tokenize(str string, cursor *Position) []Token {
	results := make([]Token, 0, 1)
	for _, group := range tokenizerRE.FindAllStringSubmatch(str, -1) {
		groupConsumed := group[0]
		if groupConsumed == "" {
			continue
		}
		if groupConsumed[0] == '\n' {
			cursor.Row++
			cursor.Col = 1
		}
		groupTrimmed := group[1]
		if (groupTrimmed == "") || (groupTrimmed[0] == ';') {
			continue
		}
		var colDelta int
		cursor.BeginCol = cursor.Col
		cursor.BeginRow = cursor.Row
		if strings.HasPrefix(groupTrimmed, "¬") {
			for _, c := range groupConsumed {
				colDelta++
				if c == '\n' {
					cursor.Row++
					colDelta = 1
				}
			}
		} else {
			colDelta = len(groupTrimmed)
		}
		cursor.Col += colDelta
		results = append(results, Token{
			Value:  groupTrimmed,
			Cursor: *cursor,
		})
		// fmt.Printf("%s⇒%s\n", cursor, groupTrimmed)
	}
	return results
}

func read_atom(rdr Reader) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, NewMalError(errors.New("read_atom underflow"), &tokenStruct)
	}
	token := &tokenStruct.Value
	if match := integerRE.MatchString(*token); match {
		var i int
		var e error
		if i, e = strconv.Atoi(*token); e != nil {
			return nil, NewMalError(errors.New("number parse error"), &tokenStruct)
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
		return nil, NewMalError(errors.New("expected '\"', got EOF"), &tokenStruct)
	} else if match := jsonStringRE.MatchString(*token); match {
		str := (*token)[2 : len(*token)-2]
		return strings.Replace(str, `¬¬`, `¬`, -1), nil
	} else if (*token)[0] == '¬' {
		return nil, NewMalError(errors.New("expected '¬', got EOF"), &tokenStruct)
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

func read_list(rdr Reader, start string, end string, placeholderValues *HashMap) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, NewMalError(errors.New("read_list underflow"), &tokenStruct)
	}
	cursor := tokenStruct.Cursor.Copy()
	token := &tokenStruct.Value
	if *token != start {
		return nil, NewMalError(errors.New("expected '"+start+"'"), &tokenStruct)
	}
	lastKnown := tokenStruct

	ast_list := []MalType{}
	tokenStruct = rdr.peek()
	for ; true; tokenStruct = rdr.peek() {
		if tokenStruct == nil {
			return nil, NewMalError(errors.New("expected '"+end+"', got EOF"), lastKnown)
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
	return List{Val: ast_list, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
}

func read_vector(rdr Reader, placeholderValues *HashMap) (MalType, error) {
	lst, e := read_list(rdr, "[", "]", placeholderValues)
	if e != nil {
		return nil, e
	}
	vec := Vector{Val: lst.(List).Val, Cursor: lst.(List).Cursor}
	return vec, nil
}

func read_hash_map(rdr Reader, placeholderValues *HashMap) (MalType, error) {
	mal_lst, e := read_list(rdr, "{", "}", placeholderValues)
	if e != nil {
		return nil, e
	}
	return NewHashMap(mal_lst)
}

func read_set(rdr Reader, placeholderValues *HashMap) (MalType, error) {
	mal_lst, e := read_list(rdr, "#{", "}", placeholderValues)
	if e != nil {
		return nil, e
	}
	return NewSet(mal_lst)
}

func read_placeholder(rdr Reader, placeholderValues *HashMap) (MalType, error) {
	tokenStruct := rdr.next()
	if tokenStruct == nil {
		return nil, NewMalError(errors.New("read_placeholder underflow"), &tokenStruct)
	}
	return placeholderValues.Val[tokenStruct.Value], nil
}

func read_form(rdr Reader, placeholderValues *HashMap) (MalType, error) {
	tokenStruct := rdr.peek()
	if tokenStruct == nil {
		return nil, NewMalError(errors.New("read_form underflow"), &tokenStruct)
	}
	cursor := tokenStruct.Cursor.Copy()
	switch tokenStruct.Value {
	case `'`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "quote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case "`":
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "quasiquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `~`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "unquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `~@`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "splice-unquote", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
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
		return List{Val: []MalType{Symbol{Val: "with-meta", Cursor: &tokenStruct.Cursor}, form, meta}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil
	case `@`:
		rdr.next()
		form, e := read_form(rdr, placeholderValues)
		if e != nil {
			return nil, e
		}
		return List{Val: []MalType{Symbol{Val: "deref", Cursor: &tokenStruct.Cursor}, form}, Cursor: cursor.Close(&tokenStruct.Cursor)}, nil

	// list
	case ")":
		return nil, NewMalError(errors.New("unexpected ')'"), &tokenStruct)
	case "(":
		return read_list(rdr, "(", ")", placeholderValues)

	// vector
	case "]":
		return nil, NewMalError(errors.New("unexpected ']'"), &tokenStruct)
	case "[":
		return read_vector(rdr, placeholderValues)

	// hash-map
	case "}":
		return nil, NewMalError(errors.New("unexpected '}'"), &tokenStruct)
	case "{":
		return read_hash_map(rdr, placeholderValues)
	case "#{":
		return read_set(rdr, placeholderValues)
	default:
		if len(tokenStruct.Value) > 0 && tokenStruct.Value[0] == '$' {
			return read_placeholder(rdr, placeholderValues)
		}
		return read_atom(rdr)
	}
}

// ";; $MODULE ../../examples/fibonacci.lisp\n(do (do\n    (def fib\n
var moduleNamePrefixRE = regexp.MustCompile(`^;; [$]MODULE (.+)`)

func Read_str(str string, cursor *Position, placeholderValues *HashMap) (MalType, error) {
	if cursor == nil {
		cursor = NewAnonymousCursorHere(1, 1)
	}
	if cursor.Module == nil {
		matches := moduleNamePrefixRE.FindStringSubmatch(str)
		if matches != nil {
			cursor = NewCursorFile(matches[1])
		}
	}
	var tokens = tokenize(str, cursor)
	if len(tokens) == 0 {
		return nil, errors.New("<empty line>")
	}

	tokenReader := TokenReader{
		tokens:   tokens,
		position: 0,
	}
	res, err := read_form(&tokenReader, placeholderValues)
	if err != nil {
		return nil, err
	}
	if tokenReader.position != len(tokenReader.tokens) {
		return nil, NewMalError(errors.New("not all tokens where parsed"), cursor)
	}
	return res, nil
}
