package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	spew "github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	call2 "github.com/jig/lisp/lib/call-helper2"
	"github.com/jig/lisp/marshaler"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"
	"github.com/jig/lisp/types"

	. "github.com/jig/lisp/types"
)

func Load(env EnvType) {
	call2.Call(env, assoc_in)
	call2.Call(env, update)
	call2.Call(env, update_in)
	call2.CallOverrideFN(env, "<", func(a, b int) (bool, error) { return a < b, nil })
	call2.CallOverrideFN(env, "<=", func(a, b int) (bool, error) { return a <= b, nil })
	call2.CallOverrideFN(env, ">", func(a, b int) (bool, error) { return a > b, nil })
	call2.CallOverrideFN(env, ">=", func(a, b int) (bool, error) { return a >= b, nil })
	call2.CallOverrideFN(env, "+", func(a, b int) (int, error) { return a + b, nil })
	call2.CallOverrideFN(env, "-", func(a, b int) (int, error) { return a - b, nil })
	call2.CallOverrideFN(env, "*", func(a, b int) (int, error) { return a * b, nil })
	call2.CallOverrideFN(env, "/", func(a, b int) (int, error) { return a / b, nil })
	call2.Call(env, get)
	call2.Call(env, get_in)
	call2.CallOverrideFN(env, "contains?", func(seq MalType, key string) (MalType, error) { return contains_Q(seq, key) })
	call2.Call(env, cons)
	call2.Call(env, nth)
	call2.Call(env, with_meta)
	call2.CallOverrideFN(env, "reset!", reset_BANG)
	call2.Call(env, rAnge)
	call2.Call(env, hash_map_decode)
	call2.Call(env, JSON_Decode)
	call2.Call(env, mErge)
	call2.Call(env, rename_keys)
	call2.Call(env, split)
	call2.Call(env, mAp)
	call2.Call(env, throw)
	call2.CallOverrideFN(env, "symbol", func(a MalType) (MalType, error) { return Symbol{Val: a.(string)}, nil })
	call2.CallOverrideFN(env, "keyword", func(a MalType) (MalType, error) {
		if Keyword_Q(a) {
			return a, nil
		} else {
			return NewKeyword(a.(string))
		}
	})
	call2.Call(env, sPew)
	call2.CallOverrideFN(env, "read-string", func(a MalType) (MalType, error) { return reader.Read_str(a.(string), nil, nil) })
	call2.CallOverrideFN(env, "set", func(a MalType) (MalType, error) { return NewSet(a) })
	call2.Call(env, keys)
	call2.Call(env, vals)
	call2.Call(env, vec)
	call2.Call(env, first)
	call2.Call(env, rest)
	call2.Call(env, count)
	call2.Call(env, seq)
	call2.Call(env, meta)
	call2.CallOverrideFN(env, "atom", func(a MalType) (MalType, error) { return &Atom{Val: a}, nil })
	call2.Call(env, deref)
	call2.Call(env, bAse64)
	call2.Call(env, unbase64)
	call2.Call(env, str2binary)
	call2.Call(env, binary2str)
	call2.Call(env, json_encode)
	call2.Call(env, sleep)
	call2.Call(env, time_ms)
	call2.Call(env, time_ns)
	call2.Call(env, uUid)
	call2.Call(env, pr_str)
	call2.Call(env, str)
	call2.Call(env, prn)
	call2.Call(env, println)
	call2.CallOverrideFN(env, "list", func(a ...MalType) (MalType, error) { return List{Val: a}, nil })
	call2.CallOverrideFN(env, "vector", func(a ...MalType) (MalType, error) { return Vector{Val: a}, nil })
	call2.Call(env, hash_map)
	call2.CallOverrideFN(env, "hash-set", func(a ...MalType) (MalType, error) { return NewSet(List{Val: a}) })
	call2.Call(env, assoc)
	call2.Call(env, dissoc)
	call2.Call(env, concat)

	call2.CallOverrideFN(env, "=", func(a, b MalType) (MalType, error) { return Equal_Q(a, b), nil })

	call2.CallOverrideFN(env, "nil?", func(a MalType) (MalType, error) { return Nil_Q(a), nil })
	call2.CallOverrideFN(env, "true?", func(a MalType) (MalType, error) { return True_Q(a), nil })
	call2.CallOverrideFN(env, "false?", func(a MalType) (MalType, error) { return False_Q(a), nil })
	call2.CallOverrideFN(env, "empty?", empty_Q)
	call2.CallOverrideFN(env, "symbol?", func(a MalType) (MalType, error) { return Q[Symbol](a), nil })
	call2.CallOverrideFN(env, "keyword?", func(a MalType) (MalType, error) { return Keyword_Q(a), nil })
	call2.CallOverrideFN(env, "string?", func(a MalType) (MalType, error) { return String_Q(a), nil })
	call2.CallOverrideFN(env, "number?", func(a MalType) (MalType, error) { return Q[int](a), nil })
	call2.CallOverrideFN(env, "fn?", fn_q)
	call2.CallOverrideFN(env, "macro?", func(a MalType) (MalType, error) { return Q[MalFunc](a) && a.(MalFunc).GetMacro(), nil })
	call2.CallOverrideFN(env, "list?", func(a MalType) (MalType, error) { return Q[List](a), nil })
	call2.CallOverrideFN(env, "vector?", func(a MalType) (MalType, error) { return Q[Vector](a), nil })
	call2.CallOverrideFN(env, "map?", func(a MalType) (MalType, error) { return Q[HashMap](a), nil })
	call2.CallOverrideFN(env, "set?", func(a MalType) (MalType, error) { return Q[Set](a), nil })
	call2.CallOverrideFN(env, "atom?", func(a MalType) (MalType, error) { return Q[*Atom](a), nil })
	call2.CallOverrideFN(env, "sequential?", func(a MalType) (MalType, error) { return Sequential_Q(a), nil })

	call2.Call(env, apply)
	call2.Call(env, conj)
	call2.CallOverrideFN(env, "swap!", swap_BANG)
	call2.Call(env, assert)
	// "apply": call.CallVeC(2, 1000_000, apply), // at least 2
	// "conj": call.CallVe(2, 1000_000, conj), // at least 2
	// "swap!": call.CallNeC(swap_BANG),
	// "assert": call.CallVe(1, 2, assert),
}

func LoadInput(env types.EnvType) {
	call2.Call(env, slurp)
	call2.Call(env, readLine)
}

// var NS = map[string]MalType{}

// func init() {
// 	call.CallTNO3e(NS, "assoc-in", assoc_in)
// 	call.CallT3eC(NS, update)
// 	call.CallTNO3eC(NS, "update-in", update_in)

// 	call.CallTNO2e(NS, "<", func(a, b int) (bool, error) { return a < b, nil })
// 	call.CallTNO2e(NS, "<=", func(a, b int) (bool, error) { return a <= b, nil })
// 	call.CallTNO2e(NS, ">", func(a, b int) (bool, error) { return a > b, nil })
// 	call.CallTNO2e(NS, ">=", func(a, b int) (bool, error) { return a >= b, nil })
// 	call.CallTNO2e(NS, "+", func(a, b int) (int, error) { return a + b, nil })
// 	call.CallTNO2e(NS, "-", func(a, b int) (int, error) { return a - b, nil })
// 	call.CallTNO2e(NS, "*", func(a, b int) (int, error) { return a * b, nil })
// 	call.CallTNO2e(NS, "/", func(a, b int) (int, error) { return a / b, nil })
// 	call.CallT2e(NS, get)
// 	call.CallTNO2e(NS, "get-in", get_in)
// 	call.CallTNO2e(NS, "contains?", func(seq MalType, key string) (MalType, error) { return contains_Q(seq, key) })
// 	call.CallT2e(NS, cons)
// 	call.CallT2e(NS, nth)
// 	call.CallT2e(NS, with_meta)
// 	call.CallTNO2e(NS, "reset!", reset_BANG)
// 	call.CallTNO2e(NS, "range", rAnge)
// 	call.CallT2e(NS, hash_map_decode)
// 	call.CallTNO2e(NS, "json-decode", JSON_Decode)
// 	call.CallTNO2e(NS, "merge", mErge)
// 	call.CallT2e(NS, rename_keys)
// 	call.CallT2e(NS, split)

// 	call.CallTNO2eC(NS, "map", mAp)

// 	call.CallT1e(NS, throw)
// 	call.CallTNO1e(NS, "symbol", func(a MalType) (MalType, error) { return Symbol{Val: a.(string)}, nil })
// 	call.CallTNO1e(NS, "keyword", func(a MalType) (MalType, error) {
// 		if Keyword_Q(a) {
// 			return a, nil
// 		} else {
// 			return NewKeyword(a.(string))
// 		}
// 	})
// 	call.CallTNO1e(NS, "spew", sPew)
// 	call.CallTNO1e(NS, "read-string", func(a MalType) (MalType, error) { return reader.Read_str(a.(string), nil, nil) })
// 	call.CallTNO1e(NS, "set", func(a MalType) (MalType, error) { return NewSet(a) })
// 	call.CallT1e(NS, keys)
// 	call.CallT1e(NS, vals)
// 	call.CallT1e(NS, vec)
// 	call.CallT1e(NS, first)
// 	call.CallT1e(NS, rest)
// 	call.CallT1e(NS, count)
// 	call.CallT1e(NS, seq)
// 	call.CallT1e(NS, meta)
// 	call.CallTNO1e(NS, "atom", func(a MalType) (MalType, error) { return &Atom{Val: a}, nil })
// 	call.CallT1e(NS, deref)
// 	call.CallTNO1e(NS, "base64", bAse64)
// 	call.CallTNO1e(NS, "unbase64", unbase64)
// 	call.CallT1e(NS, str2binary)
// 	call.CallT1e(NS, binary2str)
// 	call.CallT1e(NS, json_encode)
// 	call.CallT1eC(NS, sleep)

// 	call.CallT0e(NS, time_ms)
// 	call.CallT0e(NS, time_ns)
// 	call.CallTNO0e(NS, "uuid", uUid)

// 	call.CallTNe(NS, pr_str)
// 	call.CallTNe(NS, str)
// 	call.CallTNe(NS, prn)
// 	call.CallTNe(NS, println)
// 	call.CallTNONe(NS, "list", func(a ...MalType) (MalType, error) {
// 		return List{Val: a}, nil
// 	})
// 	call.CallTNONe(NS, "vector", func(a ...MalType) (MalType, error) { return Vector{Val: a}, nil })
// 	call.CallTNe(NS, hash_map)
// 	call.CallTNONe(NS, "hash-set", func(a ...MalType) (MalType, error) { return NewSet(List{Val: a}) })
// 	call.CallTNe(NS, assoc)
// 	call.CallTNe(NS, dissoc)
// 	call.CallTNe(NS, concat)

// 	// NSInput namespace
// 	call.CallT1e(NSInput, slurp)
// 	call.CallT1e(NSInput, readLine)
// }

// var NSInput = map[string]MalType{}

// Errors/Exceptions
func throw(a MalType) (MalType, error) {
	return nil, MalError{Obj: a}
}

func fn_q(a MalType) (MalType, error) {
	switch f := a.(type) {
	case MalFunc:
		return !f.GetMacro(), nil
	case Func:
		return true, nil
	case func([]MalType) (MalType, error):
		return true, nil
	default:
		return false, nil
	}
}

// String functions

func pr_str(a ...MalType) (MalType, error) {
	return printer.Pr_list(a, true, "", "", " "), nil
}

func str(a ...MalType) (string, error) {
	return printer.Pr_list(a, false, "", "", ""), nil
}

func sPew(a MalType) (MalType, error) {
	spew.Dump(a)
	return nil, nil
}

func prn(a ...MalType) (MalType, error) {
	fmt.Println(printer.Pr_list(a, true, "", "", " "))
	return nil, nil
}

func println(a ...MalType) (MalType, error) {
	fmt.Println(printer.Pr_list(a, false, "", "", " "))
	return nil, nil
}

func slurp(fileName string) (MalType, error) {
	b, e := os.ReadFile(fileName)
	if e != nil {
		return nil, e
	}
	return string(b), nil
}

// Number functions
func time_ms() (int, error) {
	return int(time.Now().UnixMilli()), nil
}

func time_ns() (int, error) {
	return int(time.Now().UnixNano()), nil
}

// Hash Map, Set, Vector functions
func copy_hash_map(hm HashMap) HashMap {
	new_hm := HashMap{Val: map[string]MalType{}}
	for k, v := range hm.Val {
		new_hm.Val[k] = v
	}
	return new_hm
}

func copy_set(s Set) Set {
	new_s := Set{Val: map[string]struct{}{}}
	for k, v := range s.Val {
		new_s.Val[k] = v
	}
	return new_s
}

func copy_vector(v Vector) Vector {
	return Vector{
		Val: append([]MalType{}, v.Val...),
	}
}

func assoc(a ...MalType) (MalType, error) {
	ms := a[0]
	switch ms := ms.(type) {
	case HashMap:
		if len(a) < 3 {
			return nil, errors.New("assoc requires at least 3 arguments")
		}
		if len(a)%2 != 1 {
			return nil, errors.New("assoc requires odd number of arguments")
		}
		new_hm := copy_hash_map(ms)
		for i := 1; i < len(a); i += 2 {
			key := a[i]
			if !Q[string](key) {
				return nil, errors.New("assoc called with non-string key")
			}
			new_hm.Val[key.(string)] = a[i+1]
		}
		return new_hm, nil
	case Vector:
		if len(a) < 3 {
			return nil, errors.New("assoc requires at least 3 arguments")
		}
		new_v := copy_vector(ms)
		for i := 1; i < len(a); i += 2 {
			key := a[i]
			keyInt, ok := key.(int)
			if !ok {
				return nil, errors.New("assoc called with non-int key")
			}
			new_v.Val[keyInt] = a[i+1]
		}
		return new_v, nil
	case Set:
		if len(a) < 2 {
			return nil, errors.New("assoc requires at least 2 arguments")
		}
		new_s := copy_set(ms)
		for _, value := range a[1:] {
			if !Q[string](value) {
				return nil, errors.New("assoc called with non-string key")
			}
			new_s.Val[value.(string)] = struct{}{}
		}
		return new_s, nil
	default:
		return nil, fmt.Errorf("assoc called on non-hash map and non-set (it was %T)", ms)
	}
}

func dissoc(a ...MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("dissoc requires at least 3 arguments")
	}
	ms := a[0]
	switch ms := ms.(type) {
	case HashMap:
		new_hm := copy_hash_map(ms)
		for i := 1; i < len(a); i += 1 {
			key := a[i]
			if !Q[string](key) {
				return nil, errors.New("dissoc called with non-string key")
			}
			delete(new_hm.Val, key.(string))
		}
		return new_hm, nil
	case Set:
		new_s := copy_set(ms)
		for _, value := range a[1:] {
			if !Q[string](value) {
				return nil, errors.New("dissoc called with non-string key")
			}
			delete(new_s.Val, value.(string))
		}
		return new_s, nil
	default:
		return nil, errors.New("assoc called on non-hash map and non-set")
	}
}

func get(hm, key MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	switch key.(type) {
	case string:
	case int:
	default:
		return nil, errors.New("get called with non-string key nor a non-int key")
	}
	ms := hm
	switch ms := ms.(type) {
	case HashMap:
		return ms.Val[key.(string)], nil
	case Vector:
		return ms.Val[key.(int)], nil
	case List:
		return ms.Val[key.(int)], nil
	case Set:
		if _, ok := ms.Val[key.(string)]; ok {
			return key.(string), nil
		}
		return nil, nil
	default:
		return nil, errors.New("get called on non-hash map and a non-set")
	}
}

func get_in(hm, _pathVector MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	pathVector, ok := _pathVector.(Vector)
	if !ok {
		return nil, errors.New("get-in index must be a vector")
	}
	return _getIn(hm, pathVector)
}

func _getIn(argMapOrVector MalType, posVector Vector) (MalType, error) {
	switch len(posVector.Val) {
	case 0:
		return argMapOrVector, nil
	case 1:
		index := posVector.Val[0]
		return get(argMapOrVector, index)
	default:
		index := posVector.Val[0]
		rest := Vector{Val: posVector.Val[1:]}
		var branch MalType
		switch argMapOrVector := argMapOrVector.(type) {
		case HashMap:
			branch = argMapOrVector.Val[index.(string)]
			if branch == nil {
				branch = HashMap{}
			}
		case List:
			branch = argMapOrVector.Val[index.(int)]
			if branch == nil {
				branch = List{}
			}
		case Vector:
			branch = argMapOrVector.Val[index.(int)]
			if branch == nil {
				branch = Vector{}
			}
		}
		return _getIn(branch, rest)
	}
}

func update(ctx context.Context, hm, pos, f MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	return _update(ctx, hm, pos, f)
}

func _update(ctx context.Context, argMapOrVector, index, f MalType) (MalType, error) {
	switch argMapOrVector := argMapOrVector.(type) {
	case HashMap:
		res, err := Apply(ctx, f, []MalType{argMapOrVector.Val[index.(string)]})
		if err != nil {
			return nil, err
		}
		return assoc(argMapOrVector, index, res)
	case Vector:
		res, err := Apply(ctx, f, []MalType{argMapOrVector.Val[index.(int)]})
		if err != nil {
			return nil, err
		}
		return assoc(argMapOrVector, index, res)
	default:
		return nil, fmt.Errorf("expected vector or hash-map but got %T", argMapOrVector)
	}
}

func update_in(ctx context.Context, seq MalType, posVector Vector, f MalType) (MalType, error) {
	if Nil_Q(seq) {
		return nil, nil
	}
	return _updateIn(ctx, seq, posVector, f)
}

func _updateIn(ctx context.Context, seq MalType, posVector Vector, f MalType) (MalType, error) {
	switch len(posVector.Val) {
	case 0:
		return seq, nil
	case 1:
		index := posVector.Val[0]
		return _update(ctx, seq, index, f)
	default:
		index := posVector.Val[0]
		rest := Vector{Val: posVector.Val[1:]}
		var branch MalType
		switch seq := seq.(type) {
		case HashMap:
			branch = seq.Val[index.(string)]
			if branch == nil {
				branch = HashMap{}
			}
			inner, err := _updateIn(ctx, branch.(HashMap), rest, f)
			if err != nil {
				return nil, err
			}
			return assoc(seq, index, inner)
		case Vector:
			branch = seq.Val[index.(int)]
			if branch == nil {
				branch = Vector{}
			}
			inner, err := _updateIn(ctx, branch.(Vector), rest, f)
			if err != nil {
				return nil, err
			}
			return assoc(seq, index, inner)
		default:
			return nil, fmt.Errorf("type %T not supported of index of %T", index, seq)
		}
	}
}

func assoc_in(hm MalType, posVector Vector, data MalType) (MalType, error) {
	return _assocIn(hm, posVector, data)
}

func _assocIn(argMapOrVector MalType, posVector Vector, newValue MalType) (MalType, error) {
	switch len(posVector.Val) {
	case 0:
		return argMapOrVector, nil
	case 1:
		index := posVector.Val[0]
		return assoc(argMapOrVector, index, newValue)
	default:
		index := posVector.Val[0]
		rest := Vector{Val: posVector.Val[1:]}
		var branch MalType
		switch argMapOrVector := argMapOrVector.(type) {
		case HashMap:
			branch = argMapOrVector.Val[index.(string)]
			if branch == nil {
				branch = HashMap{}
			}
		case Vector:
			branch = argMapOrVector.Val[index.(int)]
			if branch == nil {
				branch = Vector{}
			}
		}
		inner, err := _assocIn(branch, rest, newValue)
		if err != nil {
			return nil, err
		}
		return assoc(argMapOrVector, index, inner)
	}
}

func contains_Q(hm MalType, key string) (bool, error) {
	if Nil_Q(hm) {
		return false, nil
	}
	switch hm := hm.(type) {
	case HashMap:
		_, ok := hm.Val[key]
		return ok, nil
	case Set:
		_, ok := hm.Val[key]
		return ok, nil
	default:
		return false, errors.New("get called on non-hash map and a non-set")
	}
}

func keys(hm MalType) (MalType, error) {
	switch hm := hm.(type) {
	case HashMap:
		slc := []MalType{}
		for k := range hm.Val {
			slc = append(slc, k)
		}
		return List{Val: slc}, nil
	default:
		return nil, errors.New("keys called on non-hash map")
	}
}

func vals(hm MalType) (MalType, error) {
	if !Q[HashMap](hm) {
		return nil, errors.New("keys called on non-hash map")
	}
	slc := []MalType{}
	for _, v := range hm.(HashMap).Val {
		slc = append(slc, v)
	}
	return List{Val: slc}, nil
}

// Sequence functions

func cons(seq, app MalType) (MalType, error) {
	lst, e := GetSlice(app)
	if e != nil {
		return nil, e
	}
	return List{Val: append([]MalType{seq}, lst...)}, nil
}

func concat(a ...MalType) (MalType, error) {
	if len(a) == 0 {
		return List{}, nil
	}
	slc1, e := GetSlice(a[0])
	if e != nil {
		return nil, e
	}
	for i := 1; i < len(a); i += 1 {
		slc2, e := GetSlice(a[i])
		if e != nil {
			return nil, e
		}
		slc1 = append(slc1, slc2...)
	}
	return List{Val: slc1}, nil
}

func vec(seq MalType) (MalType, error) {
	array, meta, err := ConvertFrom(seq)
	if err != nil {
		return nil, err
	}
	return Vector{
		Val:  array,
		Meta: meta,
	}, nil
}

func nth(seq MalType, idx int) (MalType, error) {
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if idx < len(slc) {
		return slc[idx], nil
	} else {
		return nil, errors.New("nth: index out of range")
	}
}

func first(seq MalType) (MalType, error) {
	if seq == nil {
		return nil, nil
	}
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if len(slc) == 0 {
		return nil, nil
	}
	return slc[0], nil
}

func rest(seq MalType) (MalType, error) {
	if seq == nil {
		return List{}, nil
	}
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if len(slc) == 0 {
		return List{}, nil
	}
	return List{Val: slc[1:]}, nil
}

func empty_Q(seq MalType) (MalType, error) {
	switch seq := seq.(type) {
	case List:
		return len(seq.Val) == 0, nil
	case Vector:
		return len(seq.Val) == 0, nil
	case HashMap:
		return len(seq.Val) == 0, nil
	case Set:
		return len(seq.Val) == 0, nil
	case nil:
		return true, nil
	default:
		return nil, errors.New("empty? called on non-sequence")
	}
}

func count(seq MalType) (MalType, error) {
	switch seq := seq.(type) {
	case List:
		return len(seq.Val), nil
	case Vector:
		return len(seq.Val), nil
	case HashMap:
		return len(seq.Val), nil
	case Set:
		return len(seq.Val), nil
	case nil:
		return 0, nil
	default:
		return nil, fmt.Errorf("count called on non-sequence type %T", seq)
	}
}

func apply(ctx context.Context, a ...MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("apply requires at least 2 args")
	}
	f := a[0]
	args := append(
		[]MalType{},
		a[1:len(a)-1]...,
	)
	last, e := GetSlice(a[len(a)-1])
	if e != nil {
		return nil, e
	}
	args = append(args, last...)
	return Apply(ctx, f, args)
}

func mAp(ctx context.Context, f, seq MalType) (MalType, error) {
	results := []MalType{}
	args, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	for _, arg := range args {
		res, e := Apply(ctx, f, []MalType{arg})
		if e != nil {
			return nil, e
		}
		results = append(results, res)
	}
	return List{Val: results}, nil
}

func conj(a ...MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("conj requires at least 2 arguments")
	}
	seq := a[0]
	switch seq := seq.(type) {
	case List:
		new_slc := []MalType{}
		for i := len(a) - 1; i > 0; i -= 1 {
			new_slc = append(new_slc, a[i])
		}
		return List{Val: append(new_slc, seq.Val...)}, nil
	case Vector:
		new_slc := append(seq.Val, a[1:]...)
		return Vector{Val: new_slc}, nil
	case HashMap:
		if len(a)%2 != 1 {
			return nil, errors.New("conj called with on a hash map requires an odd number of arguments")
		}
		new_hm := copy_hash_map(seq)
		for i := 1; i < len(a); i += 2 {
			key := a[i]
			if !Q[string](key) {
				return nil, errors.New("conj called with non-string key")
			}
			new_hm.Val[key.(string)] = a[i+1]
		}
		return new_hm, nil
	case Set:
		new_s := copy_set(seq)
		for _, key := range a[1:] {
			if !Q[string](key) {
				return nil, errors.New("conj called with non-string key")
			}
			new_s.Val[key.(string)] = struct{}{}
		}
		return new_s, nil
	default:
		return nil, errors.New("conj called on non-hash map and a non-list and a non-set and a non-vector")
	}
}

func seq(seq MalType) (MalType, error) {
	switch arg := seq.(type) {
	case List:
		if len(arg.Val) == 0 {
			return nil, nil
		}
		return arg, nil
	case Vector:
		if len(arg.Val) == 0 {
			return nil, nil
		}
		return List{Val: arg.Val}, nil
	case Set:
		slc := []MalType{}
		for k := range arg.Val {
			slc = append(slc, k)
		}
		return List{Val: slc}, nil
	case string:
		if len(arg) == 0 {
			return nil, nil
		}
		new_slc := []MalType{}
		for _, ch := range strings.Split(arg, "") {
			new_slc = append(new_slc, ch)
		}
		return List{Val: new_slc}, nil
	}
	return nil, errors.New("seq requires string or list or vector or nil")
}

// Metadata functions
func with_meta(obj, meta MalType) (MalType, error) {
	switch tobj := obj.(type) {
	case List:
		return List{Val: tobj.Val, Meta: meta}, nil
	case Vector:
		return Vector{Val: tobj.Val, Meta: meta}, nil
	case HashMap:
		return HashMap{Val: tobj.Val, Meta: meta}, nil
	case Set:
		return Set{Val: tobj.Val, Meta: meta}, nil
	case Func:
		return Func{Fn: tobj.Fn, Meta: meta}, nil
	case MalFunc:
		fn := tobj
		fn.Meta = meta
		return fn, nil
	default:
		return nil, errors.New("with-meta not supported on type")
	}
}

func meta(meta MalType) (MalType, error) {
	switch meta := meta.(type) {
	case List:
		return meta.Meta, nil
	case Vector:
		return meta.Meta, nil
	case HashMap:
		return meta.Meta, nil
	case Set:
		return meta.Meta, nil
	case Func:
		return meta.Meta, nil
	case MalFunc:
		return meta.Meta, nil
	default:
		return nil, errors.New("meta not supported on type")
	}
}

// Atom functions
func deref(atomRef MalType) (MalType, error) {
	if !Q[*Atom](atomRef) {
		return nil, errors.New("deref called with non-atom")
	}
	atm := atomRef.(*Atom)
	atm.Mutex.RLock()
	defer atm.Mutex.RUnlock()
	return atm.Val, nil
}

func reset_BANG(atomRef, value MalType) (MalType, error) {
	if !Q[*Atom](atomRef) {
		return nil, errors.New("reset! called with non-atom")
	}
	atm := atomRef.(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	atm.Set(value)
	return value, nil
}

func swap_BANG(ctx context.Context, a ...MalType) (MalType, error) {
	if !Q[*Atom](a[0]) {
		return nil, errors.New("swap! called with non-atom")
	}
	atm := a[0].(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	args := []MalType{atm.Val}
	f := a[1]
	args = append(args, a[2:]...)
	res, e := Apply(ctx, f, args)
	if e != nil {
		return nil, e
	}
	atm.Set(res)
	return res, nil
}

// Core extended

func uUid() (string, error) {
	return uuid.New().String(), nil
}

func split(str, sep string) (Vector, error) {
	l := strings.Split(str, sep)
	slc := make([]MalType, len(l))
	for i, v := range l {
		slc[i] = v
	}

	return Vector{Val: slc}, nil
}

func rename_keys(data, alternative HashMap) (HashMap, error) {
	output := map[string]MalType{}
	for k, v := range data.Val {
		newKey, ok := alternative.Val[k]
		if ok {
			output[newKey.(string)] = v
		} else {
			output[k] = v
		}
	}
	return HashMap{
		Val:    output,
		Meta:   data.Meta,
		Cursor: data.Cursor,
	}, nil
}

func assert(a ...MalType) (MalType, error) {
	var a0, a1 MalType
	switch len(a) {
	case 0:
		return nil, errors.New("one or two parameters required")
	case 1:
		a0 = a[0]
	case 2:
		a0 = a[0]
		a1 = a[1]
	default:
		return nil, errors.New("one or two parameters required")
	}

	switch a0 := a0.(type) {
	case bool:
		if a0 {
			return nil, nil
		}
	default:
		return nil, nil
	case nil:
	}

	// assertion failed
	switch a1 := a1.(type) {
	case nil:
		switch a0.(type) {
		case nil:
			return nil, errors.New("assertion failed: nil")
		case bool:
			return nil, errors.New("assertion failed: false")
		default:
			return nil, errors.New("internal error")
		}
	case string:
		return nil, errors.New(a1)
	default:
		return nil, MalError{Obj: a1}
	}
}

func mErge(_hm0, _hm1 MalType) (MalType, error) {
	if _hm0 == nil && _hm1 == nil {
		return nil, nil
	}

	var hm0 HashMap
	if _hm0 != nil {
		var ok bool
		hm0, ok = _hm0.(HashMap)
		if !ok {
			return nil, errors.New("expected hash map")
		}
	}
	var hm1 HashMap
	if _hm1 != nil {
		var ok bool
		hm1, ok = _hm1.(HashMap)
		if !ok {
			return nil, errors.New("expected hash map")
		}
	}
	if hm0.Val == nil && hm1.Val == nil {
		return nil, nil
	}
	merged := HashMap{
		Val: make(map[string]MalType),
	}
	for k, v := range hm0.Val {
		merged.Val[k] = v
	}
	for k, v := range hm1.Val {
		merged.Val[k] = v
	}
	return merged, nil
}

func json_encode(obj MalType) (MalType, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func hash_map(a ...MalType) (MalType, error) {
	switch len(a) {
	case 0:
		return HashMap{}, nil
	case 1:
		return a[0].(marshaler.HashMap).MarshalHashMap()
	default:
		return NewHashMap(List{Val: a})
	}
}

func hash_map_decode(objFactory marshaler.FactoryHashMap, hm HashMap) (MalType, error) {
	return objFactory.FromHashMap(hm)
}

func JSON_Decode(obj, bytesIn MalType) (MalType, error) {
	var b []byte

	switch a := bytesIn.(type) {
	case string:
		b = []byte(a)
	case []byte:
		b = a
	default:
		return nil, fmt.Errorf("unsupported type %T", a)
	}

	switch value := obj.(type) {
	case marshaler.FactoryJSON:
		return value.FromJSON(b)
	case List:
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return array2list(v), nil
	case Vector:
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return array2vector(v), nil
	case HashMap:
		v := map[string]interface{}{}
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		err := d.Decode(&v)
		if err != nil {
			return nil, err
		}
		return map2hashmap(v), nil
	case Set:
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return NewSet(array2vector(v))
	default:
		return nil, fmt.Errorf("type %T cannot be decoded", value)
	}
}

func map2hashmap(m map[string]interface{}) MalType {
	hm := HashMap{
		Val:  map[string]MalType{},
		Meta: nil,
	}
	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			hm.Val[k] = map2hashmap(v)
		case []interface{}:
			hm.Val[k] = array2vector(v)
		default:
			hm.Val[k] = v
		}
	}
	return hm
}

func array2vector(a []interface{}) MalType {
	l := Vector{
		Val:  []MalType{},
		Meta: nil,
	}
	for _, v := range a {
		switch v := v.(type) {
		case map[string]interface{}:
			l.Val = append(l.Val, map2hashmap(v))
		case []interface{}:
			l.Val = append(l.Val, array2vector(v))
		default:
			l.Val = append(l.Val, v)
		}
	}
	return l
}

func array2list(a []interface{}) MalType {
	l := List{
		Val:  []MalType{},
		Meta: nil,
	}
	for _, v := range a {
		switch v := v.(type) {
		case map[string]interface{}:
			l.Val = append(l.Val, map2hashmap(v))
		case []interface{}:
			l.Val = append(l.Val, array2vector(v))
		default:
			l.Val = append(l.Val, v)
		}
	}
	return l
}

func readLine(prompt string) (MalType, error) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)
	scanner.Scan()
	return scanner.Text(), nil
}

func sleep(ctx context.Context, ms int) (MalType, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("timeout while evaluating expression")
	case <-time.After(time.Millisecond * time.Duration(ms)):
		return ms, nil
	}
}

func str2binary(str string) (MalType, error) {
	return []byte(str), nil
}

func binary2str(bytes MalType) (MalType, error) {
	aBytes, ok := bytes.([]byte)
	if !ok {
		return nil, errors.New("not a []byte")
	}
	return string(aBytes), nil
}

func bAse64(b []byte) (MalType, error) {
	return base64.StdEncoding.EncodeToString(b), nil
}

func unbase64(str string) (MalType, error) {
	result, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func rAnge(from, to int) (MalType, error) {
	var value []MalType
	for i := from; i < to; i++ {
		value = append(value, i)
	}
	return Vector{Val: value}, nil
}
