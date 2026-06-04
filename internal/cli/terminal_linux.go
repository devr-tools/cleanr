//go:build linux

package cli

import (
	"syscall"
	"unsafe"
)

func terminalGetAttr(fd int) (*syscall.Termios, error) {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&termios)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return &termios, nil
}

func terminalSetAttr(fd int, termios *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(termios)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
