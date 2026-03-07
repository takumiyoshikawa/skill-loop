package static

import "embed"

//go:embed all:dist
var Frontend embed.FS
