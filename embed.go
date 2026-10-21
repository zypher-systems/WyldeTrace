package wyldetrace

import "embed"

// Dist holds the built SPA assets for production serving.
//
//go:embed all:web/dist
var Dist embed.FS
