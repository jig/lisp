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
	"github.com/jig/lisp/lib/call-helper"
	"github.com/jig/lisp/marshaler"
	"github.com/jig/lisp/printer"
	"github.com/jig/lisp/reader"

	. "github.com/jig/lisp/types"
)

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

func str(a ...MalType) (MalType, error) {
	return printer.Pr_list(a, false, "", "", ""), nil
}

func spewDump(a MalType) (MalType, error) {
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

func slurp(fileName MalType) (MalType, error) {
	b, e := os.ReadFile(fileName.(string))
	if e != nil {
		return nil, e
	}
	return string(b), nil
}

// Number functions
func time_ms() (MalType, error) {
	return int(time.Now().UnixMilli()), nil
}

func time_ns() (MalType, error) {
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
			if !String_Q(key) {
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
			if !String_Q(value) {
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
			if !String_Q(key) {
				return nil, errors.New("dissoc called with non-string key")
			}
			delete(new_hm.Val, key.(string))
		}
		return new_hm, nil
	case Set:
		new_s := copy_set(ms)
		for _, value := range a[1:] {
			if !String_Q(value) {
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

func getIn(hm, _pathVector MalType) (MalType, error) {
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

func updateIn(ctx context.Context, seq, path, f MalType) (MalType, error) {
	if Nil_Q(seq) {
		return nil, nil
	}
	posVector, ok := path.(Vector)
	if !ok {
		return nil, errors.New("get called with non-vector")
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

func assocIn(hm, _posVector, data MalType) (MalType, error) {
	if Nil_Q(hm) {
		return nil, nil
	}
	posVector, ok := _posVector.(Vector)
	if !ok {
		return nil, errors.New("assoc-in called with non-vector")
	}
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

func contains_Q(hm MalType, key MalType) (MalType, error) {
	if Nil_Q(hm) {
		return false, nil
	}
	if !String_Q(key) {
		return nil, errors.New("get called with non-string key")
	}
	switch hm := hm.(type) {
	case HashMap:
		_, ok := hm.Val[key.(string)]
		return ok, nil
	case Set:
		_, ok := hm.Val[key.(string)]
		return ok, nil
	default:
		return nil, errors.New("get called on non-hash map and a non-set")
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
	if !HashMap_Q(hm) {
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

func nth(seq, pos MalType) (MalType, error) {
	slc, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	idx := pos.(int)
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
		return nil, errors.New("count called on non-sequence")
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

func do_map(ctx context.Context, f, seq MalType) (MalType, error) {
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
			if !String_Q(key) {
				return nil, errors.New("conj called with non-string key")
			}
			new_hm.Val[key.(string)] = a[i+1]
		}
		return new_hm, nil
	case Set:
		new_s := copy_set(seq)
		for _, key := range a[1:] {
			if !String_Q(key) {
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
	if !Atom_Q(atomRef) {
		return nil, errors.New("deref called with non-atom")
	}
	atm := atomRef.(*Atom)
	atm.Mutex.RLock()
	defer atm.Mutex.RUnlock()
	return atm.Val, nil
}

func reset_BANG(atomRef, value MalType) (MalType, error) {
	if !Atom_Q(atomRef) {
		return nil, errors.New("reset! called with non-atom")
	}
	atm := atomRef.(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	atm.Set(value)
	return value, nil
}

func swap_BANG(ctx context.Context, a ...MalType) (MalType, error) {
	if !Atom_Q(a[0]) {
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

// core namespace
var NS = map[string]MalType{
	"=":       call.Call2b(Equal_Q),
	"throw":   call.Call1e(throw),
	"nil?":    call.Call1b(Nil_Q),
	"true?":   call.Call1b(True_Q),
	"false?":  call.Call1b(False_Q),
	"symbol":  call.Call1e(func(a MalType) (MalType, error) { return Symbol{Val: a.(string)}, nil }),
	"symbol?": call.Call1b(Symbol_Q),
	"string?": call.Call1e(func(a MalType) (MalType, error) { return (String_Q(a) && !Keyword_Q(a)), nil }),
	"keyword": call.Call1e(func(a MalType) (MalType, error) {
		if Keyword_Q(a) {
			return a, nil
		} else {
			return NewKeyword(a.(string))
		}
	}),
	"keyword?":    call.Call1b(Keyword_Q),
	"number?":     call.Call1b(Number_Q),
	"fn?":         call.Call1e(fn_q),
	"macro?":      call.Call1e(func(a MalType) (MalType, error) { return MalFunc_Q(a) && a.(MalFunc).GetMacro(), nil }),
	"pr-str":      call.CallNe(pr_str),
	"str":         call.CallNe(str),
	"prn":         call.CallNe(prn),
	"println":     call.CallNe(println),
	"spew":        call.Call1e(spewDump),
	"read-string": call.Call1e(func(a MalType) (MalType, error) { return reader.Read_str(a.(string), nil, nil) }),
	"<":           call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) < b.(int), nil }),
	"<=":          call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) <= b.(int), nil }),
	">":           call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) > b.(int), nil }),
	">=":          call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) >= b.(int), nil }),
	"+":           call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) + b.(int), nil }),
	"-":           call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) - b.(int), nil }),
	"*":           call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) * b.(int), nil }),
	"/":           call.Call2e(func(a, b MalType) (MalType, error) { return a.(int) / b.(int), nil }),
	"time-ms":     call.Call0e(time_ms),
	"time-ns":     call.Call0e(time_ns),
	"list":        call.CallNe(func(a ...MalType) (MalType, error) { return List{Val: a}, nil }),
	"list?":       call.Call1b(List_Q),
	"vector":      call.CallNe(func(a ...MalType) (MalType, error) { return Vector{Val: a}, nil }),
	"vector?":     call.Call1b(Vector_Q),
	"hash-map":    call.CallNe(hashMap),
	"map?":        call.Call1b(HashMap_Q),
	"set":         call.Call1e(func(a MalType) (MalType, error) { return NewSet(a) }),
	"hash-set":    call.CallNe(func(a ...MalType) (MalType, error) { return NewSet(List{Val: a}) }),
	"set?":        call.Call1b(Set_Q),
	"assoc":       call.CallNe(assoc),  // at least 3
	"dissoc":      call.CallNe(dissoc), // at least 2
	"assoc-in":    call.Call3e(assocIn),
	"update":      call.Call3eC(update),
	"update-in":   call.Call3eC(updateIn),
	"get":         call.Call2e(get),
	"get-in":      call.Call2e(getIn),
	"contains?":   call.Call2e(func(seq, key MalType) (MalType, error) { return contains_Q(seq, key) }),
	"keys":        call.Call1e(keys),
	"vals":        call.Call1e(vals),
	"sequential?": call.Call1b(Sequential_Q),
	"cons":        call.Call2e(cons),
	"concat":      call.CallNe(concat),
	"vec":         call.Call1e(vec),
	"nth":         call.Call2e(nth),
	"first":       call.Call1e(first),
	"rest":        call.Call1e(rest),
	"empty?":      call.Call1e(empty_Q),
	"count":       call.Call1e(count),
	"apply":       call.CallVeC(2, 1000_000, apply), // at least 2
	"map":         call.Call2eC(do_map),
	"conj":        call.CallVe(2, 1000_000, conj), // at least 2
	"seq":         call.Call1e(seq),
	"with-meta":   call.Call2e(with_meta),
	"meta":        call.Call1e(meta),
	"atom":        call.Call1e(func(a MalType) (MalType, error) { return &Atom{Val: a}, nil }),
	"atom?":       call.Call1b(Atom_Q),
	"deref":       call.Call1e(deref),
	"reset!":      call.Call2e(reset_BANG),
	"swap!":       call.CallNeC(swap_BANG),

	"range":           call.Call2e(rangeVector),
	"sleep":           call.Call1eC(sleep),
	"base64":          call.Call1e(base64encode),
	"unbase64":        call.Call1e(base64decode),
	"str2binary":      call.Call1e(str2binary),
	"binary2str":      call.Call1e(binary2str),
	"hash-map-decode": call.Call2e(hashMapDecode),
	"json-decode":     call.Call2e(JSONDecode),
	"json-encode":     call.Call1e(jsonEncode),
	"merge":           call.Call2e(mergeHashMap),
	"assert":          call.CallVe(1, 2, assert),
	"rename-keys":     call.Call2e(renameKeys),
	"uuid":            call.Call0e(genUUID),
	"split":           call.Call2e(split),
}

var NSInput = map[string]MalType{
	"slurp":    call.Call1e(slurp),
	"readline": call.Call1e(readLine),
}

// Core extended

func genUUID() (MalType, error) {
	return uuid.New().String(), nil
}

func split(str, sep MalType) (MalType, error) {
	aStr, ok := str.(string)
	if !ok {
		return nil, errors.New("not a string")
	}

	cutSet, ok := sep.(string)
	if !ok {
		return nil, errors.New("not a string")
	}

	l := strings.Split(aStr, cutSet)
	slc := make([]MalType, len(l))
	for i, v := range l {
		slc[i] = v
	}

	return Vector{Val: slc}, nil
}

func renameKeys(_data, _alternative MalType) (MalType, error) {
	data, ok := _data.(HashMap)
	if !ok {
		return nil, errors.New("rename-keys: first parameter must be a hash-map (data input)")
	}
	alternative, ok := _alternative.(HashMap)
	if !ok {
		return nil, errors.New("rename-keys: first parameter must be a hash-map (alternative keys map)")
	}
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

func mergeHashMap(hm0, hm1 MalType) (MalType, error) {
	if hm0 == nil && hm1 == nil {
		return nil, nil
	}
	a0, ok := hm0.(HashMap)
	if !ok {
		if hm0 == nil {
			if _, ok := hm1.(HashMap); ok {
				return hm1, nil
			}
		}
		return nil, errors.New("first argument must be a map")
	}
	a1, ok := hm1.(HashMap)
	if !ok {
		if hm1 == nil {
			if _, ok := hm0.(HashMap); ok {
				return hm0, nil
			}
		}
		return nil, errors.New("second argument must be a map")
	}
	merged := HashMap{
		Val: make(map[string]MalType),
	}
	for k, v := range a0.Val {
		merged.Val[k] = v
	}
	for k, v := range a1.Val {
		merged.Val[k] = v
	}
	return merged, nil
}

func jsonEncode(obj MalType) (MalType, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func hashMap(a ...MalType) (MalType, error) {
	switch len(a) {
	case 0:
		return HashMap{}, nil
	case 1:
		return a[0].(marshaler.HashMap).MarshalHashMap()
	default:
		return NewHashMap(List{Val: a})
	}
}

func hashMapDecode(objFactory, hm MalType) (MalType, error) {
	return objFactory.(marshaler.FactoryHashMap).FromHashMap(hm)
}

func JSONDecode(obj, bytesIn MalType) (MalType, error) {
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

func readLine(_prompt MalType) (MalType, error) {
	prompt, ok := _prompt.(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)
	scanner.Scan()
	return scanner.Text(), nil
}

func sleep(ctx context.Context, _ms MalType) (MalType, error) {
	ms, ok := _ms.(int)
	if !ok {
		return nil, errors.New("not an int")
	}
	select {
	case <-ctx.Done():
		return nil, errors.New("timeout while evaluating expression")
	case <-time.After(time.Millisecond * time.Duration(ms)):
		return ms, nil
	}
}

func str2binary(str MalType) (MalType, error) {
	aStr, ok := str.(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	return []byte(aStr), nil
}

func binary2str(bytes MalType) (MalType, error) {
	aBytes, ok := bytes.([]byte)
	if !ok {
		return nil, errors.New("not a []byte")
	}
	return string(aBytes), nil
}

func base64encode(bytes MalType) (MalType, error) {
	aBytes, ok := bytes.([]byte)
	if !ok {
		return nil, errors.New("not a []byte")
	}
	return base64.StdEncoding.EncodeToString(aBytes), nil
}

func base64decode(str MalType) (MalType, error) {
	aStr, ok := str.(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	result, err := base64.StdEncoding.DecodeString(aStr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func rangeVector(from, to MalType) (MalType, error) {
	var value []MalType
	for i := from.(int); i < to.(int); i++ {
		value = append(value, i)
	}
	return Vector{Val: value}, nil
}
