// Package cli provides command-line option parsing for jig/lisp,
// modelled on clojure/tools.cli. It is pure Lisp: the namespace is a
// header evaluated into the environment (see nscli.Load), exposing
// cli-parse-opts and cli-summarize.
package cli

import _ "embed"

//go:embed header-cli.lisp
var headerCli string

func HeaderCli() string { return headerCli }
