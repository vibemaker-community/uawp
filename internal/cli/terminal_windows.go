//go:build windows

package cli

import "syscall"

func descriptorIsTerminal(fd uintptr) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(fd), &mode) == nil
}
