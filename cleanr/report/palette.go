package report

import (
	"fmt"
	"io"
	"os"
	"strings"

	"cleanr/cleanr/core"
)

type textPalette struct {
	color   bool
	accentC string
	success string
	failure string
	muted   string
	reset   string
}

func plainTextPalette() textPalette {
	return textPalette{}
}

func ansiTextPalette() textPalette {
	return textPalette{
		color:   true,
		accentC: "\x1b[38;2;0;173;181m",
		success: "\x1b[32m",
		failure: "\x1b[31m",
		muted:   "\x1b[90m",
		reset:   "\x1b[0m",
	}
}

func textPaletteForWriter(w io.Writer) textPalette {
	if !shouldColorize(w) {
		return plainTextPalette()
	}
	return ansiTextPalette()
}

func shouldColorize(w io.Writer) bool {
	if force := strings.TrimSpace(os.Getenv("FORCE_COLOR")); force != "" && force != "0" {
		return true
	}
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	if term := strings.TrimSpace(os.Getenv("TERM")); term == "" || term == "dumb" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (p textPalette) accent(text string) string {
	return p.wrap(p.accentC, text)
}

func (p textPalette) successText(text string) string {
	return p.wrap(p.success, text)
}

func (p textPalette) failureText(text string) string {
	return p.wrap(p.failure, text)
}

func (p textPalette) mutedText(text string) string {
	return p.wrap(p.muted, text)
}

func (p textPalette) wrap(colorCode, text string) string {
	if !p.color || colorCode == "" || text == "" {
		return text
	}
	return colorCode + text + p.reset
}

func (p textPalette) badge(passed bool) string {
	if passed {
		return p.successText("[PASS]")
	}
	return p.failureText("[FAIL]")
}

func (p textPalette) status(passed bool, text string) string {
	if passed {
		return p.successText(text)
	}
	return p.failureText(text)
}

func (p textPalette) failedCount(n int) string {
	value := fmt.Sprintf("%d failed", n)
	if n == 0 {
		return p.successText(value)
	}
	return p.failureText(value)
}

func writeKeyValue(b *strings.Builder, palette textPalette, label, value string) {
	fmt.Fprintf(b, "%s  %s\n", palette.accent(padRight(label, 10)), value)
}

func writeSectionHeader(b *strings.Builder, palette textPalette, title string) {
	fmt.Fprintf(b, "\n%s\n%s\n", palette.accent(title), palette.accent(strings.Repeat("-", len(title))))
}

func writeIndentedValue(b *strings.Builder, palette textPalette, indent int, label, value string) {
	fmt.Fprintf(b, "%s%s  %s\n", strings.Repeat(" ", indent), palette.mutedText(padRight(label, 7)), value)
}

func writeFinding(b *strings.Builder, palette textPalette, indent int, finding core.Finding) {
	severity := strings.ToUpper(finding.Severity)
	fmt.Fprintf(b, "%s%s  %s: %s\n", strings.Repeat(" ", indent), palette.failureText(padRight("Finding", 7)), palette.failureText(severity), finding.Message)
}

func maxSuiteNameWidth(suites []core.SuiteResult) int {
	width := len("suite")
	for _, suite := range suites {
		if len(suite.Name) > width {
			width = len(suite.Name)
		}
	}
	return width
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}
