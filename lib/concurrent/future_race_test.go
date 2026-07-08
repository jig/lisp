package concurrent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/types"
)

// TestFutureConcurrentStateNoRace exercises the Done/Cancelled flags the
// way the interpreter does: the future's goroutine writes them while
// other goroutines poll future-done?/future-cancelled? and one calls
// Cancel. Before Done/Cancelled were atomic this tripped the race
// detector (see ROADMAP-robustness.md 1.2); run with `-race`.
func TestFutureConcurrentStateNoRace(t *testing.T) {
	base := env.NewEnv()
	newFn := func() types.MalFunc {
		return types.MalFunc{
			Eval: func(_ context.Context, _ types.MalType, _ types.EnvType) (types.MalType, error) {
				time.Sleep(time.Millisecond)
				return nil, nil
			},
			Exp:    types.List{},
			Env:    base,
			Params: types.List{},
			GenEnv: env.NewSubordinateEnvWithBinds,
		}
	}

	for range 200 {
		f := NewFuture(context.Background(), newFn())

		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for range 100 {
					_ = f.Done.Load()
					_ = f.Cancelled.Load()
				}
			})
		}
		wg.Go(func() {
			f.Cancel()
		})

		// Always drains the goroutine (value or cancellation error).
		_, _ = f.Deref(context.Background())
		wg.Wait()
	}
}
