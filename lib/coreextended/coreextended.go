package coreextended

import _ "embed"

//go:embed header-coreextended.lisp
var headerCoreExtended string

func HeaderCoreExtended() string { return headerCoreExtended }
