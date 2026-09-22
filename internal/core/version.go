package core

import (
	"strconv"
	"strings"
)

const CurrentStateVersion = "1.2.0"

type StateCompatibility string

const (
	StateCurrent         StateCompatibility = "CURRENT"
	StateUpgradeRequired StateCompatibility = "UPGRADE_REQUIRED"
	StateUnsupported     StateCompatibility = "UNSUPPORTED"
	StateFutureMajor     StateCompatibility = "FUTURE_MAJOR"
)

func ClassifyStateVersion(version string) StateCompatibility {
	switch version {
	case CurrentStateVersion:
		return StateCurrent
	case "1.0.0", "1.1.0":
		return StateUpgradeRequired
	}
	majorText, _, ok := strings.Cut(version, ".")
	if !ok {
		return StateUnsupported
	}
	major, err := strconv.Atoi(majorText)
	if err != nil {
		return StateUnsupported
	}
	if major > 1 {
		return StateFutureMajor
	}
	return StateUnsupported
}
