package lisp_test

import (
	"context"
	_ "embed"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/assert/nsassert"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextented/nscoreextended"
	"github.com/jig/lisp/types"
)

//go:embed load_test.lisp
var load_test string

const (
	C = 100_000
	N = 100_000
)

func TestLoad(t *testing.T) {
	ns := env.NewEnv()

	for _, library := range []struct {
		name string
		load func(ns types.EnvType) error
	}{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs},
		{"concurrent", nsconcurrent.Load},
		{"core mal extended", nscoreextended.Load},
		{"assert", nsassert.Load},
	} {
		if err := library.load(ns); err != nil {
			log.Fatalf("Library Load Error: %v\n", err)
		}
	}

	execs := make(chan struct{})
	counter := uint32(0)

	go func() {
		for n := 0; n < N; n++ {
			execs <- struct{}{}
		}
		close(execs)
	}()

	t0 := time.Now()
	wg := sync.WaitGroup{}
	for c := 0; c < C; c++ {
		wg.Add(1)
		go func(t *testing.T) {
			for range execs {
				childNS := env.NewSubordinateEnv(ns)
				ast, err := lisp.READ(load_test, nil, childNS)
				if err != nil {
					panic(err)
				}
				res, err := lisp.EVAL(context.Background(), ast, childNS)
				if err != nil {
					panic(err)
				}
				if res != 2 {
					panic(`res != 2`)
				}
				atomic.AddUint32(&counter, 1)
			}
			wg.Done()
		}(t)
	}
	wg.Wait()

	if N != counter {
		t.Fatal(`C*N != counter`, counter)
	}
	t.Logf("%v", time.Since(t0)/N)
}
