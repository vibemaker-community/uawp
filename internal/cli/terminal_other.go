//go:build !darwin && !linux && !windows

package cli

func descriptorIsTerminal(uintptr) bool { return false }
