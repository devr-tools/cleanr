//go:build !darwin && !linux

package setup

import (
	"errors"
	"os"
)

type terminalState struct{}

func terminalUIAvailable() bool {
	return false
}

func isTerminalFile(_ *os.File) bool {
	return false
}

func makeRawTerminal(_ *os.File) (*terminalState, error) {
	return nil, errors.New("terminal UI is not supported on this platform")
}

func restoreTerminal(_ *os.File, _ *terminalState) error {
	return nil
}
