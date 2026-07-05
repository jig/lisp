// Package lazy adds lazy sequences to jig/lisp. A lazy sequence produces its
// elements on demand and memoises them, so it can be infinite (lazy-range,
// lazy-iterate, lazy-repeat, lazy-cycle) and re-traversed cheaply.
//
// The namespace is self-contained: producers and transformers return an
// opaque lazy-seq, and consumers (lazy-first, lazy-rest, lazy-nth,
// lazy-reduce) walk it. realize forces a finite lazy-seq into a vector so the
// rest of core can work on the result. Any list or vector is accepted
// wherever a lazy-seq is, so eager data flows in freely.
package lazy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

// LazySeq is a memoised thunk: forcing it yields either "empty" or a head
// plus the (lazy) tail. The thunk runs at most once; its result is cached and
// the closure released.
type LazySeq struct {
	mu     sync.Mutex
	thunk  func(ctx context.Context) (head MalType, tail *LazySeq, empty bool, err error)
	forced bool
	head   MalType
	tail   *LazySeq
	empty  bool
	err    error
}

func (l *LazySeq) LispPrint(_ func(MalType, bool) string) string { return "«lazy-seq»" }

func (l *LazySeq) force(ctx context.Context) (MalType, *LazySeq, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.forced {
		l.head, l.tail, l.empty, l.err = l.thunk(ctx)
		l.forced = true
		l.thunk = nil
	}
	return l.head, l.tail, l.empty, l.err
}

func lazy(thunk func(ctx context.Context) (MalType, *LazySeq, bool, error)) *LazySeq {
	return &LazySeq{thunk: thunk}
}

func emptySeq() *LazySeq { return &LazySeq{forced: true, empty: true} }

// toSeq views any lisp sequence (a lazy-seq, list, vector or nil) as a lazy-seq.
func toSeq(v MalType) (*LazySeq, error) {
	switch t := v.(type) {
	case *LazySeq:
		return t, nil
	case nil:
		return emptySeq(), nil
	default:
		slc, err := GetSlice(v)
		if err != nil {
			return nil, fmt.Errorf("lazy: not a sequence: %T", v)
		}
		return fromSlice(slc), nil
	}
}

func fromSlice(slc []MalType) *LazySeq {
	if len(slc) == 0 {
		return emptySeq()
	}
	return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
		return slc[0], fromSlice(slc[1:]), false, nil
	})
}

func truthy(v MalType) bool { return v != nil && v != false }

func truthyApply(ctx context.Context, pred, x MalType) (bool, error) {
	r, err := Apply(ctx, pred, []MalType{x})
	if err != nil {
		return false, err
	}
	return truthy(r), nil
}

// Load registers the lazy builtins in env.
func Load(env EnvType) {
	// producers
	call.CallOverrideFN(env, "lazy-range", lazyRange, 0, 3)
	call.CallOverrideFN(env, "lazy-iterate", lazyIterate)
	call.CallOverrideFN(env, "lazy-repeat", lazyRepeat, 1, 2)
	call.CallOverrideFN(env, "lazy-cycle", lazyCycle)
	// transformers
	call.CallOverrideFN(env, "lazy-map", lazyMap)
	call.CallOverrideFN(env, "lazy-filter", lazyFilter)
	call.CallOverrideFN(env, "lazy-remove", lazyRemove)
	call.CallOverrideFN(env, "lazy-take", lazyTake)
	call.CallOverrideFN(env, "lazy-drop", lazyDrop)
	call.CallOverrideFN(env, "lazy-take-while", lazyTakeWhile)
	call.CallOverrideFN(env, "lazy-drop-while", lazyDropWhile)
	// consumers / bridge
	call.CallOverrideFN(env, "lazy-seq", lazySeqCoerce)
	call.CallOverrideFN(env, "lazy-seq?", func(v MalType) (bool, error) { return Q[*LazySeq](v), nil })
	call.CallOverrideFN(env, "lazy-first", lazyFirst)
	call.CallOverrideFN(env, "lazy-rest", lazyRest)
	call.CallOverrideFN(env, "lazy-nth", lazyNth)
	call.CallOverrideFN(env, "lazy-reduce", lazyReduce)
	call.CallOverrideFN(env, "realize", realize)

	call.Doc(env, "lazy-range", "[] [end] [start end] [start end step]", "Lazy sequence of numbers; with no args it is infinite from 0.")
	call.Doc(env, "lazy-iterate", "[f x]", "Infinite lazy sequence x, (f x), (f (f x)), …")
	call.Doc(env, "lazy-repeat", "[x] [n x]", "Lazy sequence of x, infinite or of length n.")
	call.Doc(env, "lazy-cycle", "[coll]", "Infinite lazy repetition of coll's elements.")
	call.Doc(env, "lazy-map", "[f coll]", "Lazy sequence of (f x) for each x in coll.")
	call.Doc(env, "lazy-filter", "[pred coll]", "Lazy sequence of the items in coll for which (pred x) is truthy.")
	call.Doc(env, "lazy-remove", "[pred coll]", "Lazy sequence of the items in coll for which (pred x) is falsy.")
	call.Doc(env, "lazy-take", "[n coll]", "Lazy sequence of the first n items of coll.")
	call.Doc(env, "lazy-drop", "[n coll]", "Lazy sequence of coll without its first n items.")
	call.Doc(env, "lazy-take-while", "[pred coll]", "Lazy prefix of coll while (pred x) is truthy.")
	call.Doc(env, "lazy-drop-while", "[pred coll]", "Lazy sequence of coll after the (pred x)-truthy prefix.")
	call.Doc(env, "lazy-seq", "[coll]", "Views a list or vector as a lazy-seq.")
	call.Doc(env, "lazy-seq?", "[x]", "Whether x is a lazy-seq.")
	call.Doc(env, "lazy-first", "[coll]", "First element of coll, or nil if empty.")
	call.Doc(env, "lazy-rest", "[coll]", "Lazy sequence of coll without its first element.")
	call.Doc(env, "lazy-nth", "[coll n]", "The nth element of coll (forces up to n).")
	call.Doc(env, "lazy-reduce", "[f init coll]", "Reduces coll with f starting from init (forces coll).")
	call.Doc(env, "realize", "[coll]", "Forces a finite lazy-seq into a vector.")
}

// --- producers ---

func rangeSeq(cur, end, step int, bounded bool) *LazySeq {
	return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
		if bounded && ((step > 0 && cur >= end) || (step < 0 && cur <= end)) {
			return nil, nil, true, nil
		}
		return cur, rangeSeq(cur+step, end, step, bounded), false, nil
	})
}

func lazyRange(args ...int) (MalType, error) {
	switch len(args) {
	case 0:
		return rangeSeq(0, 0, 1, false), nil
	case 1:
		return rangeSeq(0, args[0], 1, true), nil
	case 2:
		return rangeSeq(args[0], args[1], 1, true), nil
	case 3:
		if args[2] == 0 {
			return nil, errors.New("lazy-range: step cannot be 0")
		}
		return rangeSeq(args[0], args[1], args[2], true), nil
	default:
		return nil, errors.New("lazy-range: too many arguments")
	}
}

func lazyIterate(f, x MalType) (MalType, error) {
	var mk func(x MalType) *LazySeq
	mk = func(x MalType) *LazySeq {
		return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
			tail := lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
				nxt, err := Apply(ctx, f, []MalType{x})
				if err != nil {
					return nil, nil, false, err
				}
				return mk(nxt).force(ctx)
			})
			return x, tail, false, nil
		})
	}
	return mk(x), nil
}

func lazyRepeat(params ...MalType) (MalType, error) {
	var mk func(n int, unbounded bool) *LazySeq
	mk = func(n int, unbounded bool) *LazySeq {
		if !unbounded && n <= 0 {
			return emptySeq()
		}
		return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
			return params[len(params)-1], mk(n-1, unbounded), false, nil
		})
	}
	switch len(params) {
	case 1:
		return mk(0, true), nil
	case 2:
		n, ok := params[0].(int)
		if !ok {
			return nil, errors.New("lazy-repeat: count must be an integer")
		}
		return mk(n, false), nil
	default:
		return nil, errors.New("lazy-repeat: expects (x) or (n x)")
	}
}

func lazyCycle(coll MalType) (MalType, error) {
	orig, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	var mk func(cur *LazySeq) *LazySeq
	mk = func(cur *LazySeq) *LazySeq {
		return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
			h, t, empty, err := cur.force(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			if empty {
				// wrap around; an empty source yields an empty cycle
				h, t, empty, err = orig.force(ctx)
				if err != nil {
					return nil, nil, false, err
				}
				if empty {
					return nil, nil, true, nil
				}
			}
			return h, mk(t), false, nil
		})
	}
	return mk(orig), nil
}

// --- transformers ---

func lazyMap(f, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	var mk func(src *LazySeq) *LazySeq
	mk = func(src *LazySeq) *LazySeq {
		return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
			h, t, empty, err := src.force(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			if empty {
				return nil, nil, true, nil
			}
			mapped, err := Apply(ctx, f, []MalType{h})
			if err != nil {
				return nil, nil, false, err
			}
			return mapped, mk(t), false, nil
		})
	}
	return mk(src), nil
}

func filterSeq(pred MalType, src *LazySeq, keep bool) *LazySeq {
	return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
		cur := src
		for {
			if err := ctx.Err(); err != nil {
				return nil, nil, false, err
			}
			h, t, empty, err := cur.force(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			if empty {
				return nil, nil, true, nil
			}
			ok, err := truthyApply(ctx, pred, h)
			if err != nil {
				return nil, nil, false, err
			}
			if ok == keep {
				return h, filterSeq(pred, t, keep), false, nil
			}
			cur = t
		}
	})
}

func lazyFilter(pred, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	return filterSeq(pred, src, true), nil
}

func lazyRemove(pred, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	return filterSeq(pred, src, false), nil
}

func takeSeq(n int, src *LazySeq) *LazySeq {
	if n <= 0 {
		return emptySeq()
	}
	return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
		h, t, empty, err := src.force(ctx)
		if err != nil {
			return nil, nil, false, err
		}
		if empty {
			return nil, nil, true, nil
		}
		return h, takeSeq(n-1, t), false, nil
	})
}

func lazyTake(n int, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	return takeSeq(n, src), nil
}

func lazyDrop(n int, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
		cur := src
		for range n {
			_, t, empty, err := cur.force(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			if empty {
				return nil, nil, true, nil
			}
			cur = t
		}
		return cur.force(ctx)
	}), nil
}

func lazyTakeWhile(pred, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	var mk func(src *LazySeq) *LazySeq
	mk = func(src *LazySeq) *LazySeq {
		return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
			h, t, empty, err := src.force(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			if empty {
				return nil, nil, true, nil
			}
			ok, err := truthyApply(ctx, pred, h)
			if err != nil {
				return nil, nil, false, err
			}
			if !ok {
				return nil, nil, true, nil
			}
			return h, mk(t), false, nil
		})
	}
	return mk(src), nil
}

func lazyDropWhile(pred, coll MalType) (MalType, error) {
	src, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	return lazy(func(ctx context.Context) (MalType, *LazySeq, bool, error) {
		cur := src
		for {
			if err := ctx.Err(); err != nil {
				return nil, nil, false, err
			}
			h, t, empty, err := cur.force(ctx)
			if err != nil {
				return nil, nil, false, err
			}
			if empty {
				return nil, nil, true, nil
			}
			ok, err := truthyApply(ctx, pred, h)
			if err != nil {
				return nil, nil, false, err
			}
			if !ok {
				return h, t, false, nil
			}
			cur = t
		}
	}), nil
}

// --- consumers / bridge ---

func lazySeqCoerce(coll MalType) (MalType, error) {
	s, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func lazyFirst(ctx context.Context, coll MalType) (MalType, error) {
	s, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	h, _, empty, err := s.force(ctx)
	if err != nil {
		return nil, err
	}
	if empty {
		return nil, nil
	}
	return h, nil
}

func lazyRest(ctx context.Context, coll MalType) (MalType, error) {
	s, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	_, t, empty, err := s.force(ctx)
	if err != nil {
		return nil, err
	}
	if empty {
		return emptySeq(), nil
	}
	return t, nil
}

func lazyNth(ctx context.Context, coll MalType, idx int) (MalType, error) {
	if idx < 0 {
		return nil, errors.New("lazy-nth: negative index")
	}
	cur, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	for range idx {
		_, t, empty, err := cur.force(ctx)
		if err != nil {
			return nil, err
		}
		if empty {
			return nil, errors.New("lazy-nth: index out of range")
		}
		cur = t
	}
	h, _, empty, err := cur.force(ctx)
	if err != nil {
		return nil, err
	}
	if empty {
		return nil, errors.New("lazy-nth: index out of range")
	}
	return h, nil
}

func lazyReduce(ctx context.Context, f, init, coll MalType) (MalType, error) {
	cur, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	acc := init
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h, t, empty, err := cur.force(ctx)
		if err != nil {
			return nil, err
		}
		if empty {
			return acc, nil
		}
		acc, err = Apply(ctx, f, []MalType{acc, h})
		if err != nil {
			return nil, err
		}
		cur = t
	}
}

func realize(ctx context.Context, coll MalType) (MalType, error) {
	cur, err := toSeq(coll)
	if err != nil {
		return nil, err
	}
	out := []MalType{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h, t, empty, err := cur.force(ctx)
		if err != nil {
			return nil, err
		}
		if empty {
			return Vector{Val: out}, nil
		}
		out = append(out, h)
		cur = t
	}
}
