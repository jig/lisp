module github.com/jig/lisp

go 1.25

// v0.3.0 was tagged prematurely: the 0.3 development line was not
// ready for release. With both versions retracted, @latest resolves
// back to v0.2.24.
retract (
	v0.3.0 // Published prematurely; use v0.2.24.
	v0.3.1 // Contains only this retraction.
)

require (
	github.com/chzyer/readline v1.5.1
	github.com/davecgh/go-spew v1.1.1
	github.com/eiannone/keyboard v0.0.0-20220611211555-0d226195f203
	github.com/fatih/color v1.18.0
	github.com/google/uuid v1.6.0
	github.com/jig/scanner v1.2.0
)

require (
	github.com/alexflint/go-arg v1.6.1 // indirect
	github.com/alexflint/go-scalar v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.40.0 // indirect
)
