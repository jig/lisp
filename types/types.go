package types

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type Position struct {
	Module   *string
	BeginRow int
	BeginCol int
	Row      int
	Col      int
}

func NewCursorFile(module string) *Position {
	return &Position{
		Module:   &module,
		BeginRow: 1,
		BeginCol: 1,
	}
}

func NewCursor(here *Position) *Position {
	return &Position{
		Module:   here.Module,
		BeginRow: here.BeginRow,
		BeginCol: here.BeginCol,
	}
}

func (c *Position) Close(here *Position) *Position {
	return &Position{
		Module:   c.Module,
		BeginRow: c.BeginRow,
		BeginCol: c.BeginCol,
		Row:      here.Row,
		Col:      here.Col,
	}
}

func (cursor *Position) String() string {
	if cursor == nil {
		return ""
	}
	moduleName := ""
	if cursor.Module != nil {
		moduleName = *cursor.Module
	}
	if cursor.BeginRow != cursor.Row {
		return fmt.Sprintf("%s§%d…%d", moduleName, cursor.BeginRow, cursor.Row)
	} else {
		return fmt.Sprintf("%s§%d,%d…%d", moduleName, cursor.Row, cursor.BeginCol, cursor.Col)
	}
}

type Token struct {
	Value  string
	Cursor Position
}

func (token Token) GetPosition() *Position {
	return &token.Cursor
}

// General types
type MalType interface{}

type EnvType interface {
	Find(key Symbol) EnvType
	Set(key Symbol, value MalType) MalType
	Get(key Symbol) (MalType, error)
	Remove(key Symbol) error
	RemoveNT(key Symbol) error
	Map() (map[string]interface{}, *sync.RWMutex)
	Update(key Symbol, f func(MalType) (MalType, error)) (MalType, error)

	FindNT(key Symbol) EnvType
	SetNT(key Symbol, value MalType) MalType
	GetNT(key Symbol) (MalType, error)
}

// Scalars
func Nil_Q(obj MalType) bool {
	return obj == nil
}

func True_Q(obj MalType) bool {
	b, ok := obj.(bool)
	return ok && b
}

func False_Q(obj MalType) bool {
	b, ok := obj.(bool)
	return ok && !b
}

func Q[T any](obj MalType) bool {
	_, ok := obj.(T)
	return ok
}

// Symbols
type Symbol struct {
	Val    string
	Cursor *Position
}

// Keywords
func NewKeyword(s string) (MalType, error) {
	return "\u029e" + s, nil
}

func Keyword_Q(obj MalType) bool {
	return Q[string](obj) && strings.HasPrefix(obj.(string), "\u029e")
}

func String_Q(obj MalType) bool {
	return Q[string](obj) && !strings.HasPrefix(obj.(string), "\u029e")
}

type ExternalCall func(context.Context, []MalType) (MalType, error)

// Functions
type Func struct {
	// Fn     func(context.Context, []MalType) (MalType, error)
	Fn     ExternalCall
	Meta   MalType
	Cursor *Position
}

type MalFunc struct {
	Eval    func(context.Context, MalType, EnvType) (MalType, error)
	Exp     MalType
	Env     EnvType
	Params  MalType
	IsMacro bool
	GenEnv  func(EnvType, MalType, MalType) (EnvType, error)
	Meta    MalType
	Cursor  *Position
}

func (f MalFunc) SetMacro() MalType {
	f.IsMacro = true
	return f
}

func (f MalFunc) GetMacro() bool {
	return f.IsMacro
}

// Take either a MalFunc or regular function and apply it to the
// arguments
func Apply(ctx context.Context, f_mt MalType, a []MalType) (MalType, error) {
	switch f := f_mt.(type) {
	case MalFunc:
		env, e := f.GenEnv(f.Env, f.Params, List{
			Val:    a,
			Cursor: f.Cursor,
		})
		if e != nil {
			return nil, e
		}
		return f.Eval(ctx, f.Exp, env)
	case Func:
		return f.Fn(ctx, a)
	case func([]MalType) (MalType, error):
		return f(a)
	default:
		return nil, fmt.Errorf("invalid function to Apply (%T)", f)
	}
}

// Lists
type List struct {
	Val    []MalType
	Meta   MalType
	Cursor *Position
}

func NewList(a ...MalType) MalType {
	return List{Val: a}
}

// Vectors
type Vector struct {
	Val    []MalType
	Meta   MalType
	Cursor *Position
}

func GetSlice(seq MalType) ([]MalType, error) {
	switch seq := seq.(type) {
	case List:
		return seq.Val, nil
	case Vector:
		return seq.Val, nil
	default:
		return nil, errors.New("GetSlice called on non-sequence")
	}
}

// Hash Maps
type HashMap struct {
	Val    map[string]MalType
	Meta   MalType
	Cursor *Position
}

func NewHashMap(seq MalType) (MalType, error) {
	lst, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if len(lst)%2 == 1 {
		return nil, errors.New("odd number of arguments to NewHashMap")
	}
	m := map[string]MalType{}
	for i := 0; i < len(lst); i += 2 {
		str, ok := lst[i].(string)
		if !ok {
			return nil, fmt.Errorf("expected hash-map key string (found %T)", lst[i])
		}
		m[str] = lst[i+1]
	}
	return HashMap{Val: m}, nil
}

// Sets
type Set struct {
	Val    map[string]struct{}
	Meta   MalType
	Cursor *Position
}

func NewSet(seq MalType) (MalType, error) {
	if seq == nil {
		return Set{}, nil
	}

	lst, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}

	m := map[string]struct{}{}
	for _, item := range lst {
		sItem, ok := item.(string)
		if !ok {
			return nil, errors.New("set items must be strings or keywords")
		}
		m[sItem] = struct{}{}
	}
	return Set{Val: m}, nil
}

// Dereferable type
type Dereferable interface {
	Deref(context.Context) (MalType, error)
}

// Atoms
type Atom struct {
	Mutex  sync.RWMutex
	Val    MalType
	Meta   MalType
	Cursor *Position
}

func (a *Atom) Set(val MalType) MalType {
	a.Val = val
	return a
}

func (a *Atom) Deref(_ context.Context) (MalType, error) {
	a.Mutex.RLock()
	defer a.Mutex.RUnlock()
	return a.Val, nil
}

// Future
type Future struct {
	ValChan    chan MalType
	ErrChan    chan error
	CancelFunc context.CancelFunc
	Done       bool
	Cancelled  bool

	Meta   MalType
	Cursor *Position
}

func NewFuture(ctx context.Context, fn MalFunc) *Future {
	ctx, cancel := context.WithCancel(ctx)
	f := &Future{
		ValChan:    make(chan MalType, 1),
		ErrChan:    make(chan error, 1),
		CancelFunc: cancel,
	}
	go func() {
		defer func() { f.Done = true }()
		res, err := Apply(ctx, fn, nil)
		if err != nil {
			f.ErrChan <- err
			return
		}
		f.ValChan <- res
	}()

	return f
}

func (f *Future) Cancel() bool {
	if !f.Done {
		f.Cancelled = true
		f.Done = true
		f.CancelFunc()
	}
	return f.Cancelled
}

func (f *Future) Deref(ctx context.Context) (MalType, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("timeout while dereferencing future")
	case err := <-f.ErrChan:
		f.ErrChan <- err
		return nil, err
	case res := <-f.ValChan:
		f.ValChan <- res
		return res, nil
	}
}

func Sequential_Q(seq MalType) bool {
	if seq == nil {
		return false
	}
	return (reflect.TypeOf(seq).Name() == "List") ||
		(reflect.TypeOf(seq).Name() == "Vector")
}

func Equal_Q(a, b MalType) bool {
	ota := reflect.TypeOf(a)
	otb := reflect.TypeOf(b)
	if !((ota == otb) || (Sequential_Q(a) && Sequential_Q(b))) {
		return false
	}
	//av := reflect.ValueOf(a); bv := reflect.ValueOf(b)
	//fmt.Printf("here2: %#v\n", reflect.TypeOf(a).Name())
	//switch reflect.TypeOf(a).Name() {
	switch a.(type) {
	case Symbol:
		return a.(Symbol).Val == b.(Symbol).Val
	case List:
		as, _ := GetSlice(a)
		bs, _ := GetSlice(b)
		if len(as) != len(bs) {
			return false
		}
		for i := 0; i < len(as); i += 1 {
			if !Equal_Q(as[i], bs[i]) {
				return false
			}
		}
		return true
	case Vector:
		as, _ := GetSlice(a)
		bs, _ := GetSlice(b)
		if len(as) != len(bs) {
			return false
		}
		for i := 0; i < len(as); i += 1 {
			if !Equal_Q(as[i], bs[i]) {
				return false
			}
		}
		return true
	case HashMap:
		am := a.(HashMap).Val
		bm := b.(HashMap).Val
		if len(am) != len(bm) {
			return false
		}
		for k, v := range am {
			if !Equal_Q(v, bm[k]) {
				return false
			}
		}
		return true
	case Set:
		am := a.(Set).Val
		bm := b.(Set).Val
		if len(am) != len(bm) {
			return false
		}
		for key := range am {
			if _, ok := bm[key]; !ok {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func (hm HashMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(hm.Val)
}

func (v Vector) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Val)
}

func (l List) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.Val)
}

func (s Set) MarshalJSON() ([]byte, error) {
	keys, _, err := ConvertFrom(s)
	if err != nil {
		return nil, err
	}
	return json.Marshal(keys)
}

func ConvertFrom(from MalType) ([]MalType, MalType, error) {
	switch from := from.(type) {
	case Set:
		keys := make([]MalType, 0, len(from.Val))
		for k := range from.Val {
			keys = append(keys, k)
		}
		return keys, from.Meta, nil
	case List:
		return from.Val, from.Meta, nil
	case Vector:
		return from.Val, from.Meta, nil
	default:
		return nil, nil, fmt.Errorf("cannot convert from type %T", from)
	}
}

func ConvertTo(from []MalType, _to MalType, meta MalType) (MalType, error) {
	switch _to.(type) {
	case Set:
		to := Set{Val: map[string]struct{}{}}
		for _, k := range from {
			to.Val[k.(string)] = struct{}{}
		}
		return to, nil
	case List:
		return List{
			Val:    from,
			Meta:   meta,
			Cursor: &Position{},
		}, nil
	case Vector:
		return Vector{
			Val:    from,
			Meta:   meta,
			Cursor: &Position{},
		}, nil
	default:
		return nil, fmt.Errorf("cannot convert to type %T", _to)
	}
}

func Line(cursor *Position, message string) string {
	return cursor.String() + ": " + message
}

// Placeholder
type Placeholder struct {
	Index  int
	Cursor *Position
}
