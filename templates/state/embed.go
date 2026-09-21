package state

import _ "embed"

//go:embed CONTEXT.md
var Context []byte

//go:embed ACTIVE_WORKER.md
var ActiveWorker []byte

//go:embed DECISIONS.md
var Decisions []byte
