//go:build darwin || linux

package cli

import (
	"os"
	"syscall"
)

type terminalState struct {
	state syscall.Termios
}

func terminalUIAvailable() bool {
	return true
}

func isTerminalFile(f *os.File) bool {
	if f == nil {
		return false
	}
	_, err := terminalGetAttr(int(f.Fd()))
	return err == nil
}

func makeRawTerminal(f *os.File) (*terminalState, error) {
	termios, err := terminalGetAttr(int(f.Fd()))
	if err != nil {
		return nil, err
	}

	raw := *termios
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := terminalSetAttr(int(f.Fd()), &raw); err != nil {
		return nil, err
	}

	return &terminalState{state: *termios}, nil
}

func restoreTerminal(f *os.File, state *terminalState) error {
	if f == nil || state == nil {
		return nil
	}
	return terminalSetAttr(int(f.Fd()), &state.state)
}
