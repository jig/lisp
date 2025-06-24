package lisp

import (
	"github.com/jig/lisp/types"
)

// Debug is an interface for debugging evaluation of Lisp expressions.
type Debug interface {
	Wait(msg DebugMessage) (cmd DebugControl)
	Reset()
	PushFile(filename, contents string)

	DoNotStopStatus() bool
	CancelStatus() bool
	SetDoNotStop(bool) (previousDoNotStopStatus bool)
	SetCancelStatus(bool) (previousCancelStatus bool)
}

//go:generate stringer -type=DebugControl -linecomment
type DebugControl int

const (
	DebugNoop DebugControl = iota
	DebugStepOver
	DebugStepInto
	DebugStepOut
	DebugContinue
	DebugExit
)

type DebugMessage struct {
	Env      types.EnvType
	Input    types.MalType
	Result   types.MalType
	Err      error
	Filename *string
	Contents *string
}
