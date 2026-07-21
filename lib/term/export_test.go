package term

import (
	"os"
	"sync"
)

// ResetColorCache re-evaluates the color decisions on next use, so tests
// can flip the environment between cases.
func ResetColorCache() {
	colorEnabled = sync.OnceValue(func() bool { return colorDecisionFor(os.Stdout.Fd()) })
	colorEnabledStderr = sync.OnceValue(func() bool { return colorDecisionFor(os.Stderr.Fd()) })
}
