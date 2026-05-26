package setup

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type tuiSession struct {
	in  *os.File
	out *os.File
}

type tuiKey int

const (
	tuiKeyUnknown tuiKey = iota
	tuiKeyEnter
	tuiKeyUp
	tuiKeyDown
	tuiKeyBackspace
	tuiKeyCtrlC
)

func (t tuiSession) ask(label, fallback string) (string, error) {
	return t.readTextStep(label, fallback, false, false)
}

func (t tuiSession) askRequired(label, fallback string) (string, error) {
	return t.readTextStep(label, fallback, false, true)
}

func (t tuiSession) askChoice(label, fallback string, options []string) (string, error) {
	defaultIndex := 0
	for i, option := range options {
		if option == strings.TrimSpace(fallback) {
			defaultIndex = i
			break
		}
	}

	selected := defaultIndex
	var value string
	err := t.withRawTerminal(func() error {
		for {
			t.renderChoiceScreen(label, options, selected)
			key, _, err := t.readKey()
			if err != nil {
				return err
			}
			switch key {
			case tuiKeyUp:
				selected = (selected - 1 + len(options)) % len(options)
			case tuiKeyDown:
				selected = (selected + 1) % len(options)
			case tuiKeyEnter:
				value = options[selected]
				return nil
			case tuiKeyCtrlC:
				return io.EOF
			}
		}
	})
	return value, err
}

func (t tuiSession) askSecret(label, fallback string) (string, error) {
	return t.readTextStep(label, fallback, true, true)
}

func (t tuiSession) confirmOpenBrowser(providerName, url string, force bool) error {
	openBrowser := force
	var err error
	if !force {
		choice, chooseErr := t.askChoice(
			fmt.Sprintf("Open browser for %s key setup?", providerName),
			"open browser",
			[]string{"open browser", "skip"},
		)
		if chooseErr != nil {
			return chooseErr
		}
		openBrowser = choice == "open browser"
	}

	if !openBrowser {
		return nil
	}

	if err = openBrowserURL(url); err != nil {
		return t.showMessage(
			"Browser Launch Failed",
			fmt.Sprintf("Could not launch your browser.\n\nVisit this URL manually:\n%s\n\nError: %v\n\nPress Enter to continue.", url, err),
		)
	}
	return t.showMessage(
		"Browser Opened",
		fmt.Sprintf("Finish login and create an API key for %s in your browser.\n\nURL:\n%s\n\nReturn here when you are ready, then press Enter.", providerName, url),
	)
}

func (t tuiSession) readTextStep(label, fallback string, secret, required bool) (string, error) {
	value := []rune{}
	var out string
	err := t.withRawTerminal(func() error {
		for {
			t.renderTextScreen(label, fallback, string(value), secret, required)
			key, r, err := t.readKey()
			if err != nil {
				return err
			}
			switch key {
			case tuiKeyEnter:
				out = string(value)
				if strings.TrimSpace(out) == "" {
					out = strings.TrimSpace(fallback)
				}
				if required && strings.TrimSpace(out) == "" {
					continue
				}
				return nil
			case tuiKeyBackspace:
				if len(value) > 0 {
					value = value[:len(value)-1]
				}
			case tuiKeyCtrlC:
				return io.EOF
			default:
				if r >= 32 && r <= 126 {
					value = append(value, r)
				}
			}
		}
	})
	return out, err
}

func (t tuiSession) showMessage(title, body string) error {
	return t.withRawTerminal(func() error {
		for {
			t.clearScreen()
			_, _ = fmt.Fprintf(t.out, "cleanr setup\n\n%s\n\n%s\n", title, body)
			key, _, err := t.readKey()
			if err != nil {
				return err
			}
			if key == tuiKeyEnter {
				return nil
			}
			if key == tuiKeyCtrlC {
				return io.EOF
			}
		}
	})
}

func (t tuiSession) renderChoiceScreen(label string, options []string, selected int) {
	t.clearScreen()
	_, _ = fmt.Fprintf(t.out, "cleanr setup\n\n%s\n\n", label)
	for i, option := range options {
		prefix := "  "
		if i == selected {
			prefix = "› "
		}
		_, _ = fmt.Fprintf(t.out, "%s%s\n", prefix, option)
	}
	_, _ = fmt.Fprint(t.out, "\nUse ↑ and ↓ to move, then press Enter.\n")
}

func (t tuiSession) renderTextScreen(label, fallback, value string, secret, required bool) {
	t.clearScreen()
	visible := value
	if secret {
		visible = strings.Repeat("*", len([]rune(value)))
	}

	_, _ = fmt.Fprintf(t.out, "cleanr setup\n\n%s\n\n", label)
	if visible == "" && strings.TrimSpace(fallback) != "" {
		if secret {
			_, _ = fmt.Fprint(t.out, "(default hidden)\n")
		} else {
			_, _ = fmt.Fprintf(t.out, "Default: %s\n", fallback)
		}
	}
	if required {
		_, _ = fmt.Fprint(t.out, "Required.\n")
	}
	_, _ = fmt.Fprintf(t.out, "\n> %s", visible)
	_, _ = fmt.Fprint(t.out, "\n\nType to edit, Backspace to delete, Enter to continue.\n")
}

func (t tuiSession) withRawTerminal(fn func() error) error {
	state, err := makeRawTerminal(t.in)
	if err != nil {
		return err
	}
	defer func() {
		_ = restoreTerminal(t.in, state)
		_, _ = fmt.Fprint(t.out, "\x1b[?25h\x1b[0m\x1b[2J\x1b[H")
	}()

	_, _ = fmt.Fprint(t.out, "\x1b[?25l")
	return fn()
}

func (t tuiSession) clearScreen() {
	_, _ = fmt.Fprint(t.out, "\x1b[2J\x1b[H")
}

func (t tuiSession) readKey() (tuiKey, rune, error) {
	var buf [1]byte
	if _, err := t.in.Read(buf[:]); err != nil {
		return tuiKeyUnknown, 0, err
	}

	switch buf[0] {
	case '\r', '\n':
		return tuiKeyEnter, 0, nil
	case 3:
		return tuiKeyCtrlC, 0, nil
	case 127, 8:
		return tuiKeyBackspace, 0, nil
	case 27:
		var seq [2]byte
		if _, err := t.in.Read(seq[:]); err != nil {
			return tuiKeyUnknown, 0, nil
		}
		if seq[0] == '[' {
			switch seq[1] {
			case 'A':
				return tuiKeyUp, 0, nil
			case 'B':
				return tuiKeyDown, 0, nil
			}
		}
		return tuiKeyUnknown, 0, nil
	default:
		return tuiKeyUnknown, rune(buf[0]), nil
	}
}
