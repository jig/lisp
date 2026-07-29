// Package version identifies the running program: the (version)
// builtin returns the binary's build information as a hash-map, and
// the Go API lets an embedder brand its own REPL or binary.
//
// By default the identity comes from Go build info: :main names the
// main module (the embedder's module when jig/lisp is embedded, or
// jig/lisp itself for the lisp binaries) and jig/lisp appears among
// the dependencies. An embedder that versions its product some other
// way (e.g. -ldflags -X) calls Set before loading the namespace.
package version

import (
	"runtime/debug"

	"github.com/jig/lisp/lib/call"
	. "github.com/jig/lisp/types"
)

// overrideName/overrideVersion, when set, replace the main-module
// identity everywhere it is reported.
var overrideName, overrideVersion string

// SetMain brands the running program: name and ver replace the main
// module's path and version in (version)'s :main and in --version.
// Call it before evaluating lisp code (typically from main, with
// values injected via -ldflags -X).
func SetMain(name, ver string) {
	overrideName, overrideVersion = name, ver
}

// Main returns the running program's identity: the SetMain override when
// present, otherwise the main module's path and version from build
// info ("", "" when unavailable).
func Main() (name, ver string) {
	if overrideName != "" || overrideVersion != "" {
		return overrideName, overrideVersion
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	return bi.Main.Path, bi.Main.Version
}

// moduleVersion returns the version of the module at importPath, looking
// in both the main module and the dependencies: github.com/jig/lisp is
// the main module when the binary is built from this repo, but a
// dependency when jig/lisp is embedded in another Go program.
func moduleVersion(bi *debug.BuildInfo, importPath string) string {
	if bi.Main.Path == importPath {
		return bi.Main.Version
	}
	for _, d := range bi.Deps {
		if d.Path == importPath {
			return d.Version
		}
	}
	return ""
}

// Versions returns the jig/lisp, jig/scanner and Go toolchain versions of
// the running binary, shared by the (version) builtin and --version. Each
// value is "" when build information is unavailable.
func Versions() (lispVer, scannerVer, goVer string) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", "", ""
	}
	return moduleVersion(bi, "github.com/jig/lisp"),
		moduleVersion(bi, "github.com/jig/scanner"),
		bi.GoVersion
}

func version() (HashMap, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return HashMap{}, nil
	}
	mainName, mainVersion := Main()
	build := map[MalType]MalType{}
	for _, s := range bi.Settings {
		build[s.Key] = s.Value
	}
	deps := map[MalType]MalType{}
	for _, d := range bi.Deps {
		if d.Replace == nil {
			deps[d.Path] = HashMap{Items: map[MalType]MalType{
				KW("version"): d.Version,
				KW("sum"):     d.Sum,
			}}
		} else {
			deps[d.Path] = HashMap{Items: map[MalType]MalType{
				KW("version"): d.Version,
				KW("sum"):     d.Sum,
				KW("replace"): d.Replace,
			}}
		}
	}
	return HashMap{Items: map[MalType]MalType{
		KW("main"): HashMap{Items: map[MalType]MalType{
			KW("name"):    mainName,
			KW("version"): mainVersion,
		}},
		KW("go-version"):   bi.GoVersion,
		KW("build"):        HashMap{Items: build},
		KW("dependencies"): HashMap{Items: deps},
	}}, nil
}

// Load registers the version builtin.
func Load(env EnvType) {
	call.Call(env, version)
	call.Doc(env, "version", "[]",
		"Build information of the running program as a hash-map: :main {:name :version} (the program itself — an embedder's module, or its version.SetMain branding), :go-version, :build settings and :dependencies.")
}
