package docgen

import (
	"fmt"

	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/cli/nscli"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/lib/git/nsgit"
	"github.com/jig/lisp/lib/integrity/nsintegrity"
	"github.com/jig/lisp/lib/lazy/nslazy"
	"github.com/jig/lisp/lib/log/nslog"
	"github.com/jig/lisp/lib/regexp/nsregexp"
	"github.com/jig/lisp/lib/require/nsrequire"
	"github.com/jig/lisp/lib/sql/nssql"
	"github.com/jig/lisp/lib/system/nssystem"
	"github.com/jig/lisp/lib/term/nsterm"
	"github.com/jig/lisp/lib/test/nstest"
	"github.com/jig/lisp/lib/version/nsversion"
	"github.com/jig/lisp/lib/web/nsweb"
	"github.com/jig/lisp/types"
)

// library is one namespace to document: its display name and its loader.
type library struct {
	name string
	load func(ns types.EnvType) error
}

// Library is a namespace an embedder wants documented alongside the
// standard ones. Name identifies the library in the reference; Load
// registers its symbols. Title and Desc override the section heading and
// its one-line description, which otherwise fall back to Name and none.
type Library struct {
	Name  string
	Load  func(ns types.EnvType) error
	Title string
	Desc  string
}

// standardLibraries lists the namespaces documented in LANGUAGE.md, in
// load order. It deliberately mirrors cmd/lisp's own libraries() list so
// the reference describes exactly what the binary ships; the two lists
// live in different packages (cmd/lisp is package main and cannot be
// imported), and TestDocgenLibrariesInSync (in cmd/lisp) fails if they
// drift apart.
func standardLibraries() []library {
	return []library{
		{"core mal", nscore.Load},
		{"core mal with input", nscore.LoadInput},
		{"command line args", nscore.LoadCmdLineArgs(nil)},
		{"concurrent", nsconcurrent.Load},
		{"core mal extended", nscoreextended.Load},
		{"system", nssystem.Load},
		{"lazy", nslazy.Load},
		{"require", nsrequire.Load("lisp")},
		{"sql", nssql.Load},
		{"cli", nscli.Load},
		{"integrity", nsintegrity.Load},
		{"regexp", nsregexp.Load},
		{"web", nsweb.Load},
		{"git", nsgit.Load},
		{"term", nsterm.Load},
		{"test", nstest.Load},
		{"log", nslog.Load},
		{"version", nsversion.Load},
	}
}

// StandardSymbols loads every documented library and returns the sorted
// names bound in the resulting environment. It exists so a test in the
// cmd/lisp package can assert docgen documents exactly the environment
// the binary builds.
func StandardSymbols() ([]string, error) { return SymbolsFor(nil) }

// mergeLibraries appends an embedder's libraries to the standard list,
// failing closed on a nil loader or a name that collides with a
// namespace already listed (a silent collision would hijack that
// section's heading and attribution).
func mergeLibraries(extra []Library) ([]library, error) {
	libs := standardLibraries()
	names := map[string]bool{}
	for _, l := range libs {
		names[l.name] = true
	}
	for _, e := range extra {
		if e.Load == nil {
			return nil, fmt.Errorf("docgen: library %q has a nil Load", e.Name)
		}
		if names[e.Name] {
			return nil, fmt.Errorf("docgen: library %q collides with an already listed namespace", e.Name)
		}
		names[e.Name] = true
		libs = append(libs, library{name: e.Name, load: e.Load})
	}
	return libs, nil
}

// SymbolsFor is StandardSymbols with an embedder's libraries loaded too,
// so an embedder can assert the same way that its reference documents
// exactly the environment its binary builds.
func SymbolsFor(extra []Library) ([]string, error) {
	ns := env.NewEnv()
	libs, err := mergeLibraries(extra)
	if err != nil {
		return nil, err
	}
	for _, lib := range libs {
		if err := lib.load(ns); err != nil {
			return nil, err
		}
	}
	return ns.(*env.Env).LocalSymbols(), nil
}
