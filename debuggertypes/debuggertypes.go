package debuggertypes

type Command int

const (
	NoOp Command = iota
	Next
	Out
	In
)
