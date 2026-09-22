//go:build linux

package cli

import (
	"syscall"
	"unsafe"
)

func descriptorIsTerminal(fd uintptr) bool {
	var value syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&value)), 0, 0, 0)
	return errno == 0
}
