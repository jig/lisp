package core

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jig/mal/printer"
	"github.com/jig/mal/reader"

	. "github.com/jig/mal/types"
)

// Errors/Exceptions
func throw(a []MalType) (MalType, error) {
	return nil, MalError{a[0]}
}

func fn_q(a []MalType) (MalType, error) {
	switch f := a[0].(type) {
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

func pr_str(a []MalType) (MalType, error) {
	return printer.Pr_list(a, true, "", "", " "), nil
}

func str(a []MalType) (MalType, error) {
	return printer.Pr_list(a, false, "", "", ""), nil
}

func prn(a []MalType) (MalType, error) {
	fmt.Println(printer.Pr_list(a, true, "", "", " "))
	return nil, nil
}

func println(a []MalType) (MalType, error) {
	fmt.Println(printer.Pr_list(a, false, "", "", " "))
	return nil, nil
}

func slurp(a []MalType) (MalType, error) {
	b, e := os.ReadFile(a[0].(string))
	if e != nil {
		return nil, e
	}
	return string(b), nil
}

// Number functions
func time_ms(a []MalType) (MalType, error) {
	return int(time.Now().UnixMilli()), nil
}

func time_ns(a []MalType) (MalType, error) {
	return int(time.Now().UnixNano()), nil
}

// Hash Map functions
func copy_hash_map(hm HashMap) HashMap {
	new_hm := HashMap{Val: map[string]MalType{}}
	for k, v := range hm.Val {
		new_hm.Val[k] = v
	}
	return new_hm
}

func assoc(a []MalType) (MalType, error) {
	if len(a) < 3 {
		return nil, errors.New("assoc requires at least 3 arguments")
	}
	if len(a)%2 != 1 {
		return nil, errors.New("assoc requires odd number of arguments")
	}
	if !HashMap_Q(a[0]) {
		return nil, errors.New("assoc called on non-hash map")
	}
	new_hm := copy_hash_map(a[0].(HashMap))
	for i := 1; i < len(a); i += 2 {
		key := a[i]
		if !String_Q(key) {
			return nil, errors.New("assoc called with non-string key")
		}
		new_hm.Val[key.(string)] = a[i+1]
	}
	return new_hm, nil
}

func dissoc(a []MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("dissoc requires at least 3 arguments")
	}
	if !HashMap_Q(a[0]) {
		return nil, errors.New("dissoc called on non-hash map")
	}
	new_hm := copy_hash_map(a[0].(HashMap))
	for i := 1; i < len(a); i += 1 {
		key := a[i]
		if !String_Q(key) {
			return nil, errors.New("dissoc called with non-string key")
		}
		delete(new_hm.Val, key.(string))
	}
	return new_hm, nil
}

func get(a []MalType) (MalType, error) {
	if Nil_Q(a[0]) {
		return nil, nil
	}
	if !HashMap_Q(a[0]) {
		return nil, errors.New("get called on non-hash map")
	}
	if !String_Q(a[1]) {
		return nil, errors.New("get called with non-string key")
	}
	return a[0].(HashMap).Val[a[1].(string)], nil
}

func contains_Q(hm MalType, key MalType) (MalType, error) {
	if Nil_Q(hm) {
		return false, nil
	}
	if !HashMap_Q(hm) {
		return nil, errors.New("get called on non-hash map")
	}
	if !String_Q(key) {
		return nil, errors.New("get called with non-string key")
	}
	_, ok := hm.(HashMap).Val[key.(string)]
	return ok, nil
}

func keys(a []MalType) (MalType, error) {
	if !HashMap_Q(a[0]) {
		return nil, errors.New("keys called on non-hash map")
	}
	slc := []MalType{}
	for k, _ := range a[0].(HashMap).Val {
		slc = append(slc, k)
	}
	return List{Val: slc}, nil
}

func vals(a []MalType) (MalType, error) {
	if !HashMap_Q(a[0]) {
		return nil, errors.New("keys called on non-hash map")
	}
	slc := []MalType{}
	for _, v := range a[0].(HashMap).Val {
		slc = append(slc, v)
	}
	return List{Val: slc}, nil
}

// Sequence functions

func cons(a []MalType) (MalType, error) {
	val := a[0]
	lst, e := GetSlice(a[1])
	if e != nil {
		return nil, e
	}
	return List{Val: append([]MalType{val}, lst...)}, nil
}

func concat(a []MalType) (MalType, error) {
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

func vec(a []MalType) (MalType, error) {
	switch obj := a[0].(type) {
	case Vector:
		return obj, nil
	case List:
		return Vector{Val: obj.Val}, nil
	default:
		return nil, errors.New("vec: expects a sequence")
	}
}

func nth(a []MalType) (MalType, error) {
	slc, e := GetSlice(a[0])
	if e != nil {
		return nil, e
	}
	idx := a[1].(int)
	if idx < len(slc) {
		return slc[idx], nil
	} else {
		return nil, errors.New("nth: index out of range")
	}
}

func first(a []MalType) (MalType, error) {
	if len(a) == 0 {
		return nil, nil
	}
	if a[0] == nil {
		return nil, nil
	}
	slc, e := GetSlice(a[0])
	if e != nil {
		return nil, e
	}
	if len(slc) == 0 {
		return nil, nil
	}
	return slc[0], nil
}

func rest(a []MalType) (MalType, error) {
	if a[0] == nil {
		return List{}, nil
	}
	slc, e := GetSlice(a[0])
	if e != nil {
		return nil, e
	}
	if len(slc) == 0 {
		return List{}, nil
	}
	return List{Val: slc[1:]}, nil
}

func empty_Q(a []MalType) (MalType, error) {
	switch obj := a[0].(type) {
	case List:
		return len(obj.Val) == 0, nil
	case Vector:
		return len(obj.Val) == 0, nil
	case nil:
		return true, nil
	default:
		return nil, errors.New("empty? called on non-sequence")
	}
}

func count(a []MalType) (MalType, error) {
	switch obj := a[0].(type) {
	case List:
		return len(obj.Val), nil
	case Vector:
		return len(obj.Val), nil
	case map[string]MalType:
		return len(obj), nil
	case nil:
		return 0, nil
	default:
		return nil, errors.New("count called on non-sequence")
	}
}

func apply(a []MalType, ctx *context.Context) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("apply requires at least 2 args")
	}
	f := a[0]
	args := []MalType{}
	for _, b := range a[1 : len(a)-1] {
		args = append(args, b)
	}
	last, e := GetSlice(a[len(a)-1])
	if e != nil {
		return nil, e
	}
	args = append(args, last...)
	return Apply(f, args, ctx)
}

func do_map(a []MalType, ctx *context.Context) (MalType, error) {
	f := a[0]
	results := []MalType{}
	args, e := GetSlice(a[1])
	if e != nil {
		return nil, e
	}
	for _, arg := range args {
		res, e := Apply(f, []MalType{arg}, ctx)
		results = append(results, res)
		if e != nil {
			return nil, e
		}
	}
	return List{Val: results}, nil
}

func conj(a []MalType) (MalType, error) {
	if len(a) < 2 {
		return nil, errors.New("conj requires at least 2 arguments")
	}
	switch seq := a[0].(type) {
	case List:
		new_slc := []MalType{}
		for i := len(a) - 1; i > 0; i -= 1 {
			new_slc = append(new_slc, a[i])
		}
		return List{Val: append(new_slc, seq.Val...)}, nil
	case Vector:
		new_slc := seq.Val
		for _, x := range a[1:] {
			new_slc = append(new_slc, x)
		}
		return Vector{Val: new_slc}, nil
	}

	if !HashMap_Q(a[0]) {
		return nil, errors.New("dissoc called on non-hash map")
	}
	new_hm := copy_hash_map(a[0].(HashMap))
	for i := 1; i < len(a); i += 1 {
		key := a[i]
		if !String_Q(key) {
			return nil, errors.New("dissoc called with non-string key")
		}
		delete(new_hm.Val, key.(string))
	}
	return new_hm, nil
}

func seq(a []MalType) (MalType, error) {
	if a[0] == nil {
		return nil, nil
	}
	switch arg := a[0].(type) {
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
func with_meta(a []MalType) (MalType, error) {
	obj := a[0]
	m := a[1]
	switch tobj := obj.(type) {
	case List:
		return List{Val: tobj.Val, Meta: m}, nil
	case Vector:
		return Vector{Val: tobj.Val, Meta: m}, nil
	case HashMap:
		return HashMap{Val: tobj.Val, Meta: m}, nil
	case Func:
		return Func{Fn: tobj.Fn, Meta: m}, nil
	case MalFunc:
		fn := tobj
		fn.Meta = m
		return fn, nil
	default:
		return nil, errors.New("with-meta not supported on type")
	}
}

func meta(a []MalType) (MalType, error) {
	obj := a[0]
	switch tobj := obj.(type) {
	case List:
		return tobj.Meta, nil
	case Vector:
		return tobj.Meta, nil
	case HashMap:
		return tobj.Meta, nil
	case Func:
		return tobj.Meta, nil
	case MalFunc:
		return tobj.Meta, nil
	default:
		return nil, errors.New("meta not supported on type")
	}
}

// Atom functions
func deref(a []MalType) (MalType, error) {
	if !Atom_Q(a[0]) {
		return nil, errors.New("deref called with non-atom")
	}
	atm := a[0].(*Atom)
	atm.Mutex.RLock()
	defer atm.Mutex.RUnlock()
	return atm.Val, nil
}

func reset_BANG(a []MalType) (MalType, error) {
	if !Atom_Q(a[0]) {
		return nil, errors.New("reset! called with non-atom")
	}
	atm := a[0].(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	atm.Set(a[1])
	return a[1], nil
}

func swap_BANG(a []MalType, ctx *context.Context) (MalType, error) {
	if !Atom_Q(a[0]) {
		return nil, errors.New("swap! called with non-atom")
	}
	atm := a[0].(*Atom)
	atm.Mutex.Lock()
	defer atm.Mutex.Unlock()
	args := []MalType{atm.Val}
	f := a[1]
	args = append(args, a[2:]...)
	res, e := Apply(f, args, ctx)
	if e != nil {
		return nil, e
	}
	atm.Set(res)
	return res, nil
}

// core namespace
var NS = map[string]MalType{
	"=":       Call2b(Equal_Q),
	"throw":   Call1e(throw),
	"nil?":    Call1b(Nil_Q),
	"true?":   Call1b(True_Q),
	"false?":  Call1b(False_Q),
	"symbol":  Call1e(func(a []MalType) (MalType, error) { return Symbol{a[0].(string)}, nil }),
	"symbol?": Call1b(Symbol_Q),
	"string?": Call1e(func(a []MalType) (MalType, error) { return (String_Q(a[0]) && !Keyword_Q(a[0])), nil }),
	"keyword": Call1e(func(a []MalType) (MalType, error) {
		if Keyword_Q(a[0]) {
			return a[0], nil
		} else {
			return NewKeyword(a[0].(string))
		}
	}),
	"keyword?":    Call1b(Keyword_Q),
	"number?":     Call1b(Number_Q),
	"fn?":         Call1e(fn_q),
	"macro?":      Call1e(func(a []MalType) (MalType, error) { return MalFunc_Q(a[0]) && a[0].(MalFunc).GetMacro(), nil }),
	"pr-str":      CallNe(pr_str),
	"str":         CallNe(str),
	"prn":         CallNe(prn),
	"println":     CallNe(println),
	"read-string": Call1e(func(a []MalType) (MalType, error) { return reader.Read_str(a[0].(string), nil) }),
	"<":           Call2e(func(a []MalType) (MalType, error) { return a[0].(int) < a[1].(int), nil }),
	"<=":          Call2e(func(a []MalType) (MalType, error) { return a[0].(int) <= a[1].(int), nil }),
	">":           Call2e(func(a []MalType) (MalType, error) { return a[0].(int) > a[1].(int), nil }),
	">=":          Call2e(func(a []MalType) (MalType, error) { return a[0].(int) >= a[1].(int), nil }),
	"+":           Call2e(func(a []MalType) (MalType, error) { return a[0].(int) + a[1].(int), nil }),
	"-":           Call2e(func(a []MalType) (MalType, error) { return a[0].(int) - a[1].(int), nil }),
	"*":           Call2e(func(a []MalType) (MalType, error) { return a[0].(int) * a[1].(int), nil }),
	"/":           Call2e(func(a []MalType) (MalType, error) { return a[0].(int) / a[1].(int), nil }),
	"time-ms":     Call0e(time_ms),
	"time-ns":     Call0e(time_ns),
	"list":        CallNe(func(a []MalType) (MalType, error) { return List{Val: a}, nil }),
	"list?":       Call1b(List_Q),
	"vector":      CallNe(func(a []MalType) (MalType, error) { return Vector{Val: a}, nil }),
	"vector?":     Call1b(Vector_Q),
	"hash-map":    CallNe(func(a []MalType) (MalType, error) { return NewHashMap(List{Val: a}) }),
	"map?":        Call1b(HashMap_Q),
	"assoc":       CallNe(assoc),  // at least 3
	"dissoc":      CallNe(dissoc), // at least 2
	"get":         Call2e(get),
	"contains?":   Call2e(func(a []MalType) (MalType, error) { return contains_Q(a[0], a[1]) }),
	"keys":        Call1e(keys),
	"vals":        Call1e(vals),
	"sequential?": Call1b(Sequential_Q),
	"cons":        Call2e(cons),
	"concat":      CallNe(concat),
	"vec":         Call1e(vec),
	"nth":         Call2e(nth),
	"first":       Call1e(first),
	"rest":        Call1e(rest),
	"empty?":      Call1e(empty_Q),
	"count":       Call1e(count),
	"apply":       CallNeC(apply), // at least 2
	"map":         Call2eC(do_map),
	"conj":        CallNe(conj), // at least 2
	"seq":         Call1e(seq),
	"with-meta":   Call2e(with_meta),
	"meta":        Call1e(meta),
	"atom":        Call1e(func(a []MalType) (MalType, error) { return &Atom{Val: a[0]}, nil }),
	"atom?":       Call1b(Atom_Q),
	"deref":       Call1e(deref),
	"reset!":      Call2e(reset_BANG),
	"swap!":       CallNeC(swap_BANG),

	"range":      Call2e(rangeVector),
	"sleep":      Call1eC(sleep),
	"base64":     Call1e(base64encode),
	"unbase64":   Call1e(base64decode),
	"str2binary": Call1e(str2binary),
	"binary2str": Call1e(binary2str),
	"jsondecode": Call1e(jsonDecode),
	"jsonencode": Call1e(jsonEncode),
	"merge":      Call2e(mergeHashMap),
	"assert":     CallNe(assert),
}

var NSInput = map[string]MalType{
	"slurp":    Call1e(slurp),
	"readline": Call1e(readLine),
}

// Core extended
func assert(a []MalType) (MalType, error) {
	var a0, a1 MalType
	switch len(a) {
	case 0:
		return nil, errors.New("one or two parameters required")
	case 1:
		a0 = a[0]
	case 2:
		a0 = a[0]
		a1 = a[1]
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
			return nil, errors.New("assertion failed nil")
		case bool:
			return nil, errors.New("assertion failed false")
		default:
			return nil, errors.New("internal error")
		}
	case string:
		return nil, errors.New(a1)
	default:
		return nil, MalError{Obj: a1}
	}
}

func mergeHashMap(a []MalType) (MalType, error) {
	if a[0] == nil && a[1] == nil {
		return nil, nil
	}
	a0, ok := a[0].(HashMap)
	if !ok {
		if a[0] == nil {
			if _, ok := a[1].(HashMap); ok {
				return a[1], nil
			}
		}
		return nil, errors.New("first argument must be a map")
	}
	a1, ok := a[1].(HashMap)
	if !ok {
		if a[1] == nil {
			if _, ok := a[0].(HashMap); ok {
				return a[0], nil
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

func jsonEncode(a []MalType) (MalType, error) {
	b, err := json.Marshal(a[0])
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func jsonDecode(a []MalType) (MalType, error) {
	var b []byte

	switch a := a[0].(type) {
	case string:
		b = []byte(a)
	case []byte:
		b = a
	default:
		return nil, fmt.Errorf("unsupported type %T", a)
	}

	switch b[0] {
	case '{':
		v := map[string]interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return map2hashmap(v), nil
	case '[':
		v := []interface{}{}
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return array2vector(v), nil
	default:
		var v MalType
		err := json.Unmarshal(b, &v)
		if err != nil {
			return nil, err
		}
		return v, nil
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

func readLine(a []MalType) (MalType, error) {
	prompt, ok := a[0].(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)
	scanner.Scan()
	return scanner.Text(), nil
}

func sleep(a []MalType, ctx *context.Context) (MalType, error) {
	aInt, ok := a[0].(int)
	if !ok {
		return nil, errors.New("not an int")
	}
	select {
	case <-(*ctx).Done():
		return nil, errors.New("timeout while evaluating expression")
	case <-time.After(time.Millisecond * time.Duration(aInt)):
		return aInt, nil
	}
}

func str2binary(a []MalType) (MalType, error) {
	aStr, ok := a[0].(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	return []byte(aStr), nil
}

func binary2str(a []MalType) (MalType, error) {
	aBytes, ok := a[0].([]byte)
	if !ok {
		return nil, errors.New("not a []byte")
	}
	return string(aBytes), nil
}

func base64encode(a []MalType) (MalType, error) {
	aBytes, ok := a[0].([]byte)
	if !ok {
		return nil, errors.New("not a []byte")
	}
	return base64.StdEncoding.EncodeToString(aBytes), nil
}

func base64decode(a []MalType) (MalType, error) {
	aStr, ok := a[0].(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	result, err := base64.StdEncoding.DecodeString(aStr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func rangeVector(a []MalType) (MalType, error) {
	var value []MalType
	for i := a[0].(int); i < a[1].(int); i++ {
		value = append(value, i)
	}
	return Vector{Val: value}, nil
}

// callXX functions check the number of arguments
func Call0e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0)", len(args))
		}
		return f(args)
	}
}

func Call1e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args)
	}
}

func Call2e(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args)
	}
}

func CallNe(f func([]MalType) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args)
	}
}

func Call1b(f func(MalType) bool) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args[0]), nil
	}
}

func Call2b(f func(MalType, MalType) bool) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, _ *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args[0], args[1]), nil
	}
}

func CallNeC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	// just for documenting purposes, does not check anything
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		return f(args, ctx)
	}
}

func Call0eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 0 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 0", len(args))
		}
		return f(args, ctx)
	}
}

func Call1eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 1)", len(args))
		}
		return f(args, ctx)
	}
}

func Call2eC(f func([]MalType, *context.Context) (MalType, error)) func([]MalType, *context.Context) (MalType, error) {
	return func(args []MalType, ctx *context.Context) (result MalType, err error) {
		defer malRecover(&err)
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (%d instead of 2)", len(args))
		}
		return f(args, ctx)
	}
}

func malRecover(err *error) {
	if rerr := recover(); rerr != nil {
		*err = rerr.(error)
	}
}
