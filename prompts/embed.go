package prompts

import "embed"

// FS contains the versioned canonical UAWP prompts.
//
//go:embed *.md
var FS embed.FS
