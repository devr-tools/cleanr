package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type promptSession struct {
	reader      *bufio.Reader
	out         io.Writer
	interactive bool
}

func newSetupPrompter(stdin io.Reader, stdout io.Writer, ciMode bool) setupPrompter {
	if ciMode {
		return promptSession{
			reader: bufio.NewReader(stdin),
			out:    stdout,
		}
	}

	stdinFile, stdinOK := stdin.(*os.File)
	stdoutFile, stdoutOK := stdout.(*os.File)
	if stdinOK && stdoutOK && terminalUIAvailable() && isTerminalFile(stdinFile) && isTerminalFile(stdoutFile) {
		return tuiSession{in: stdinFile, out: stdoutFile}
	}

	return promptSession{
		reader:      bufio.NewReader(stdin),
		out:         stdout,
		interactive: stdinOK && stdoutOK && isTerminalFile(stdinFile) && isTerminalFile(stdoutFile),
	}
}

func ensureWritableOutput(path string, force bool) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is required")
	}
	if force {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; rerun with -force to overwrite", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (p promptSession) ask(label, fallback string) (string, error) {
	if strings.TrimSpace(fallback) != "" {
		_, _ = fmt.Fprintf(p.out, "%s [%s]: ", label, fallback)
	} else {
		_, _ = fmt.Fprintf(p.out, "%s: ", label)
	}

	line, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" && errors.Is(err, io.EOF) {
		return "", io.EOF
	}
	return value, nil
}

func (p promptSession) askRequired(label, fallback string) (string, error) {
	value, err := p.ask(label, fallback)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}
	return value, nil
}

func (p promptSession) askChoice(label, fallback string, options []string) (string, error) {
	value, err := p.ask(label, fallback)
	if err != nil {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for _, option := range options {
		if value == option {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s must be one of %s", strings.ToLower(label), strings.Join(options, ", "))
}

func (p promptSession) askSecret(label, fallback string) (string, error) {
	return p.ask(label, fallback)
}

func (p promptSession) confirmOpenBrowser(providerName, url string, force bool) error {
	if force {
		if err := openBrowserURL(url); err != nil {
			_, _ = fmt.Fprintf(p.out, "browser open failed; visit %s manually: %v\n", url, err)
			return nil
		}
		_, _ = fmt.Fprintf(p.out, "opened browser for %s. Finish login or key creation, then return here.\n", providerName)
		return nil
	}
	if !p.interactive {
		return nil
	}

	answer, err := p.ask(fmt.Sprintf("Open browser for %s authentication? [Y/n]", providerName), "y")
	if err != nil {
		return err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "n" || answer == "no" {
		return nil
	}
	if err := openBrowserURL(url); err != nil {
		_, _ = fmt.Fprintf(p.out, "browser open failed; visit %s manually: %v\n", url, err)
		return nil
	}
	_, _ = fmt.Fprintf(p.out, "opened browser for %s. Finish login or key creation, then return here.\n", providerName)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func slugify(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return defaultAgentName
	}

	var b strings.Builder
	lastDash := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return defaultAgentName
	}
	return out
}

func maxInt(value, floor int) int {
	if value > floor {
		return value
	}
	return floor
}

func intPtr(v int) *int {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
