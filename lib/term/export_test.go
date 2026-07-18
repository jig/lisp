package term

import "sync"

// ResetColorCache re-evaluates the color decision on next use, so tests
// can flip the environment between cases.
func ResetColorCache() {
	colorEnabled = sync.OnceValue(colorDecision)
}
