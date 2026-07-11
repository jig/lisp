//go:build lispdebug

// Package debugadapter implements a Debug Adapter Protocol (DAP) server
// for the Lisp interpreter. It is compiled only into `lispdebug` builds.
//
// The server speaks DAP over stdio or a TCP socket. A VSCode extension
// (or any DAP client) connects, sets breakpoints, launches the program,
// and steps through the evaluation.
//
// Spec reference: https://microsoft.github.io/debug-adapter-protocol/
package debugadapter

import "encoding/json"

// Message is the common envelope for every DAP packet. Type is
// "request", "response", or "event"; the discriminating field is read
// before the rest of the payload.
type Message struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"`
}

// Request is sent by the client.
type Request struct {
	Message
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// Response is sent by the server in reply to a Request.
type Response struct {
	Message
	RequestSeq int         `json:"request_seq"`
	Success    bool        `json:"success"`
	Command    string      `json:"command"`
	Message_   string      `json:"message,omitempty"`
	Body       interface{} `json:"body,omitempty"`
}

// Event is sent by the server to notify the client.
type Event struct {
	Message
	Event string      `json:"event"`
	Body  interface{} `json:"body,omitempty"`
}

// Capabilities advertised in the response to `initialize`.
type Capabilities struct {
	SupportsConfigurationDoneRequest bool `json:"supportsConfigurationDoneRequest"`
	SupportsTerminateRequest         bool `json:"supportsTerminateRequest"`
	SupportsStepInTargetsRequest     bool `json:"supportsStepInTargetsRequest"`
	SupportsEvaluateForHovers        bool `json:"supportsEvaluateForHovers"`
	SupportsConditionalBreakpoints   bool `json:"supportsConditionalBreakpoints"`
	SupportsLogPoints                bool `json:"supportsLogPoints"`
	SupportsSetVariable              bool `json:"supportsSetVariable"`

	ExceptionBreakpointFilters []ExceptionBreakpointsFilter `json:"exceptionBreakpointFilters,omitempty"`
}

// ExceptionBreakpointsFilter is one selectable exception category the
// client can toggle (advertised in `initialize`, toggled via
// `setExceptionBreakpoints`).
type ExceptionBreakpointsFilter struct {
	Filter  string `json:"filter"`
	Label   string `json:"label"`
	Default bool   `json:"default,omitempty"`
}

// EvaluateArguments is the payload of an `evaluate` request (Debug
// Console input, watch expressions and hovers).
type EvaluateArguments struct {
	Expression string `json:"expression"`
	FrameID    int    `json:"frameId"`
	Context    string `json:"context,omitempty"`
}

// Source identifies a script file in DAP terms.
type Source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

// SourceBreakpoint is a breakpoint requested by the client.
type SourceBreakpoint struct {
	Line int `json:"line"`
	// Condition is a lisp expression; the breakpoint fires only when it
	// evaluates to a truthy value in the paused frame's environment.
	Condition string `json:"condition,omitempty"`
	// LogMessage turns the breakpoint into a logpoint: rather than
	// pausing, its text is emitted as output with `{expr}` placeholders
	// interpolated.
	LogMessage string `json:"logMessage,omitempty"`
}

// Breakpoint is the server's confirmation back to the client.
type Breakpoint struct {
	Verified bool   `json:"verified"`
	Line     int    `json:"line,omitempty"`
	Source   Source `json:"source,omitempty"`
}

// StackFrame is one entry in the response to `stackTrace`.
type StackFrame struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Source Source `json:"source,omitempty"`
}

// Scope groups variables visible at a stack frame.
type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive"`
}

// Variable is one entry in the response to `variables`.
type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
}

// Thread describes a thread of execution. The Lisp DAP server exposes a
// single thread today.
type Thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Argument types ----------------------------------------------------------

// LaunchArguments are the parameters sent with a `launch` request.
type LaunchArguments struct {
	Program     string `json:"program,omitempty"`
	StopOnEntry bool   `json:"stopOnEntry,omitempty"`
	// Cwd is applied with os.Chdir before the program runs. VSCode
	// spawns the adapter with an arbitrary working directory (often /),
	// which would break relative load-file paths and require's
	// git-root search; the extension defaults this to the workspace
	// folder.
	Cwd string `json:"cwd,omitempty"`
}

// SetBreakpointsArguments configures breakpoints in a single source file.
type SetBreakpointsArguments struct {
	Source      Source             `json:"source"`
	Breakpoints []SourceBreakpoint `json:"breakpoints"`
}

// SetExceptionBreakpointsArguments lists the exception filters the client
// wants active (by their advertised filter id).
type SetExceptionBreakpointsArguments struct {
	Filters []string `json:"filters"`
}

// StackTraceArguments selects a thread and a window of frames.
type StackTraceArguments struct {
	ThreadID   int `json:"threadId"`
	StartFrame int `json:"startFrame"`
	Levels     int `json:"levels"`
}

// ScopesArguments selects a stack frame.
type ScopesArguments struct {
	FrameID int `json:"frameId"`
}

// VariablesArguments selects a variables reference (a scope or a
// composite value previously returned to the client).
type VariablesArguments struct {
	VariablesReference int `json:"variablesReference"`
}

// ContinueArguments selects a thread.
type ContinueArguments struct {
	ThreadID int `json:"threadId"`
}

// NextArguments / StepInArguments / StepOutArguments / PauseArguments all
// share the same shape today: a thread id.
type NextArguments = ContinueArguments
type StepInArguments = ContinueArguments
type StepOutArguments = ContinueArguments
type PauseArguments = ContinueArguments

// Event bodies ------------------------------------------------------------

// StoppedEventBody is sent when execution pauses.
type StoppedEventBody struct {
	Reason            string `json:"reason"`
	Description       string `json:"description,omitempty"`
	ThreadID          int    `json:"threadId"`
	AllThreadsStopped bool   `json:"allThreadsStopped"`
}

// OutputEventBody carries program output to the client.
type OutputEventBody struct {
	Category string `json:"category,omitempty"`
	Output   string `json:"output"`
}

// ExitedEventBody reports the exit code of the debuggee.
type ExitedEventBody struct {
	ExitCode int `json:"exitCode"`
}
