package types

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
)

type Token struct {
	Value  string
	Type   rune
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
	Update(key Symbol, f func(MalType) (MalType, error)) (MalType, error)
	Symbols(newLine [][]rune, lastPartial string) [][]rune

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

// Keyword is a lisp keyword (:foo). It is a distinct Go type, so a
// keyword can never be confused with a string carrying the same
// characters \u2014 unlike the historical representation (a string with a
// "\u029e" prefix), where any external string starting with that rune
// became a keyword.
type Keyword string

func (k Keyword) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(k))
}

// KW builds the keyword :s from its name (without the colon).
func KW(s string) Keyword {
	return Keyword(s)
}

// NewKeyword builds the keyword :s from its name (without the colon).
//
// Deprecated: use KW. NewKeyword returned the prefixed-string
// representation before 0.4 and is kept so embedder code keeps
// compiling; it now returns a Keyword.
func NewKeyword(s string) Keyword {
	return Keyword(s)
}

func Keyword_Q(obj MalType) bool {
	return Q[Keyword](obj)
}

func String_Q(obj MalType) bool {
	return Q[string](obj)
}

// ValidKey reports whether obj can be a hash-map key or a set element:
// any immutable scalar — nil, boolean, int, float, string or keyword.
// Restricting keys to these keeps every map operation panic-free (all
// are comparable Go types with value semantics) and gives them a total
// order (KeyLess) for deterministic sequencing and printing. Composite
// values (vectors, maps, sets), symbols and big ints are rejected: the
// first are not hashable as Go map keys, the latter two only compare
// by pointer identity, which would break lisp value equality.
func ValidKey(obj MalType) bool {
	switch obj.(type) {
	case nil, bool, int, float32, string, Keyword:
		return true
	}
	return false
}

// keyRank groups valid keys by type for KeyLess: nil, booleans,
// numbers, strings, keywords. (Strings before keywords matches the
// order the sorted legacy ʞ encoding produced.)
func keyRank(k MalType) int {
	switch k.(type) {
	case nil:
		return 0
	case bool:
		return 1
	case int:
		return 2
	case float32:
		return 3
	case string:
		return 4
	case Keyword:
		return 5
	}
	return 6
}

// KeyLess is the total order over valid hash-map keys and set elements:
// by type group (keyRank), then by value within the group.
func KeyLess(a, b MalType) bool {
	ra, rb := keyRank(a), keyRank(b)
	if ra != rb {
		return ra < rb
	}
	switch a := a.(type) {
	case bool:
		return !a && b.(bool)
	case int:
		return a < b.(int)
	case float32:
		return a < b.(float32)
	case string:
		return a < b.(string)
	case Keyword:
		return a < b.(Keyword)
	default: // nil, or invalid keys during error paths
		return false
	}
}

type ExternalCall func(context.Context, []MalType) (MalType, error)

// Functions
type Func struct {
	// Fn     func(context.Context, []MalType) (MalType, error)
	Fn   ExternalCall
	Meta MalType
	// Doc and Arglist hold documentation attached with call.Doc. They
	// are deliberately NOT exposed through `meta`, which stays nil for
	// native functions (kanaka/mal compatibility); tooling reads these
	// fields directly.
	Doc     string
	Arglist string
	Cursor  *Position
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

func NewList(cursor *Position, a ...MalType) MalType {
	return List{Val: a, Cursor: cursor}
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
	case HashMap:
		// A hash-map seqs as its entries, each a [key value] vector
		// (Clojure MapEntry), in sorted key order. Keys are returned
		// exactly as stored. The order must be deterministic — not just
		// any order: Go randomises map iteration per call, and first and
		// rest each seq the map independently, so an unstable order makes
		// them disagree and a first/rest traversal (reduce, filter) drops
		// or repeats entries. Clojure gets the same coherence from its
		// maps' stable iteration order.
		keys := sortedKeys(seq.Items)
		entries := make([]MalType, 0, len(keys))
		for _, k := range keys {
			entries = append(entries, Vector{Val: []MalType{k, seq.Items[k]}})
		}
		return entries, nil
	case Set:
		// A set seqs as its elements, as stored, in sorted order — see
		// the hash-map case for why the order must be deterministic.
		elems := make([]MalType, 0, len(seq.Items))
		for k := range seq.Items {
			elems = append(elems, k)
		}
		sort.Slice(elems, func(i, j int) bool { return KeyLess(elems[i], elems[j]) })
		return elems, nil
	default:
		return nil, errors.New("GetSlice called on non-sequence")
	}
}

// Hash Maps
type HashMap struct {
	// Items maps keys to values. Keys are restricted to immutable
	// scalars (see ValidKey); every constructor validates, so map
	// operations never hit a non-comparable key.
	Items  map[MalType]MalType
	Meta   MalType
	Cursor *Position
}

// sortedKeys returns m's keys in KeyLess order.
func sortedKeys[V any](m map[MalType]V) []MalType {
	keys := make([]MalType, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return KeyLess(keys[i], keys[j]) })
	return keys
}

func NewHashMap(cursor *Position, seq MalType) (MalType, error) {
	lst, e := GetSlice(seq)
	if e != nil {
		return nil, e
	}
	if len(lst)%2 == 1 {
		return nil, errors.New("odd number of arguments to NewHashMap")
	}
	m := map[MalType]MalType{}
	for i := 0; i < len(lst); i += 2 {
		if !ValidKey(lst[i]) {
			return nil, fmt.Errorf("hash-map keys must be scalar values — string, keyword, number, boolean or nil (found %T)", lst[i])
		}
		m[lst[i]] = lst[i+1]
	}
	return HashMap{Items: m, Cursor: cursor}, nil
}

// Sets
type Set struct {
	// Items holds the elements, restricted to string and Keyword as
	// hash-map keys are (see ValidKey).
	Items  map[MalType]struct{}
	Meta   MalType
	Cursor *Position
}

func NewSet(seq MalType) (Set, error) {
	if seq == nil {
		return Set{}, nil
	}

	lst, e := GetSlice(seq)
	if e != nil {
		return Set{}, e
	}

	m := map[MalType]struct{}{}
	for _, item := range lst {
		if !ValidKey(item) {
			return Set{}, fmt.Errorf("set items must be scalar values — string, keyword, number, boolean or nil (found %T)", item)
		}
		m[item] = struct{}{}
	}
	return Set{Items: m}, nil
}

// Dereferable type
type Dereferable interface {
	Deref(context.Context) (MalType, error)
}

// LispPrintable type
type LispPrintable interface {
	LispPrint(func(obj MalType, print_readably bool) string) string
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
		am := a.(HashMap).Items
		bm := b.(HashMap).Items
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
		am := a.(Set).Items
		bm := b.(Set).Items
		if len(am) != len(bm) {
			return false
		}
		for key := range am {
			if _, ok := bm[key]; !ok {
				return false
			}
		}
		return true
	case *big.Int:
		// Pointer values: compare numerically, not by identity.
		ab := a.(*big.Int)
		bb, _ := b.(*big.Int)
		if ab == nil || bb == nil {
			return ab == bb
		}
		return ab.Cmp(bb) == 0
	default:
		return a == b
	}
}

// MarshalJSON serialises keyword keys as their bare name (:a → "a"),
// as Clojure JSON emitters do. (Before 0.4 the internal ʞ prefix leaked
// into the JSON output.) A map holding both :x and "x" produces
// duplicate JSON keys — of which one survives, unspecified.
func (hm HashMap) MarshalJSON() ([]byte, error) {
	m := make(map[string]MalType, len(hm.Items))
	for k, v := range hm.Items {
		switch k := k.(type) {
		case string:
			m[k] = v
		case Keyword:
			m[string(k)] = v
		default:
			return nil, fmt.Errorf("cannot JSON-encode a hash-map key of type %T", k)
		}
	}
	return json.Marshal(m)
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
		keys := make([]MalType, 0, len(from.Items))
		for k := range from.Items {
			keys = append(keys, k)
		}
		return keys, from.Meta, nil
	case List:
		return from.Val, from.Meta, nil
	case Vector:
		return from.Val, from.Meta, nil
	case HashMap:
		entries, _ := GetSlice(from)
		return entries, from.Meta, nil
	default:
		return nil, nil, fmt.Errorf("cannot convert from type %T", from)
	}
}

func ConvertTo(from []MalType, _to MalType, meta MalType) (MalType, error) {
	switch _to.(type) {
	case Set:
		to := Set{Items: map[MalType]struct{}{}}
		for _, k := range from {
			if !ValidKey(k) {
				return nil, fmt.Errorf("set items must be scalar values — string, keyword, number, boolean or nil (found %T)", k)
			}
			to.Items[k] = struct{}{}
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

type Typed interface {
	Type() string
}
