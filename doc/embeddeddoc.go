package doc

import "embed"

//go:embed *.json
var AllJSONFiles embed.FS
