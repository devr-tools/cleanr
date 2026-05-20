package cleanr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

func WriteReport(w io.Writer, report Report, format string) error {
	switch strings.ToLower(format) {
	case "", "text":
		_, err := fmt.Fprint(w, renderTextReport(report, textPaletteForWriter(w)))
		return err
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "junit":
		return writeJUnit(w, report)
	default:
		return fmt.Errorf("unsupported report format: %s", format)
	}
}

func TextReport(report Report) string {
	return renderTextReport(report, plainTextPalette())
}

func renderTextReport(report Report, palette textPalette) string {
	var b strings.Builder
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "%s %s\n", palette.accent("cleanr"), palette.status(report.Passed, status))
	fmt.Fprintf(&b, "%s\n", palette.accent(strings.Repeat("=", 48)))
	writeKeyValue(&b, palette, "Target", report.Name)
	if !report.GeneratedAt.IsZero() {
		writeKeyValue(&b, palette, "Generated", report.GeneratedAt.Format(time.RFC3339))
	}
	writeKeyValue(&b, palette, "Duration", report.Duration.Round(time.Millisecond).String())
	writeKeyValue(&b, palette, "Suites", fmt.Sprintf("%d total | %s", report.TotalSuites, palette.failedCount(report.FailedSuites)))
	writeKeyValue(&b, palette, "Cases", fmt.Sprintf("%d total | %s", report.TotalCases, palette.failedCount(report.FailedCases)))

	writeSectionHeader(&b, palette, "Overview")
	suiteWidth := maxSuiteNameWidth(report.Suites)
	for _, suite := range report.Suites {
		fmt.Fprintf(&b, "%s %-*s  %s\n", palette.badge(suite.Passed), suiteWidth, suite.Name, suiteSummaryText(suite))
	}

	writeSectionHeader(&b, palette, "Details")
	for i, suite := range report.Suites {
		if i > 0 {
			fmt.Fprintln(&b)
		}
		fmt.Fprintf(&b, "%s %s\n", suite.Name, palette.badge(suite.Passed))
		if summary := suiteSummaryText(suite); summary != "" {
			writeIndentedValue(&b, palette, 2, "Summary", summary)
		}
		for _, c := range suite.Cases {
			fmt.Fprintf(&b, "  - %s %s\n", c.Name, palette.badge(c.Passed))
			if summary := caseSummaryText(c); summary != "" {
				writeIndentedValue(&b, palette, 4, "Metrics", summary)
			}
			for _, f := range c.Findings {
				writeFinding(&b, palette, 4, f)
			}
		}
		for _, f := range suite.Findings {
			writeFinding(&b, palette, 2, f)
		}
		if meta := suiteMetaText(suite.Meta); meta != "" {
			writeIndentedValue(&b, palette, 2, "Meta", meta)
		}
	}
	if len(report.Recommendations) > 0 {
		writeSectionHeader(&b, palette, "Recommendations")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(&b, "  - %s\n", rec)
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

type junitSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Time      string      `xml:"time,attr"`
	TestCases []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string        `xml:"name,attr"`
	Class   string        `xml:"classname,attr"`
	Time    string        `xml:"time,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func writeJUnit(w io.Writer, report Report) error {
	suites := junitSuites{}
	for _, suite := range report.Suites {
		js := junitSuite{
			Name:  suite.Name,
			Tests: len(suite.Cases),
			Time:  formatSeconds(suite.Duration),
		}
		for _, c := range suite.Cases {
			jc := junitCase{
				Name:  c.Name,
				Class: suite.Name,
				Time:  formatSeconds(c.Duration),
			}
			if !c.Passed {
				js.Failures++
				jc.Failure = &junitFailure{
					Message: "cleanr assertion failed",
					Body:    findingsText(c.Findings),
				}
			}
			js.TestCases = append(js.TestCases, jc)
		}
		suites.Suites = append(suites.Suites, js)
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(suites)
}

func findingsText(findings []Finding) string {
	var parts []string
	for _, f := range findings {
		parts = append(parts, strings.ToUpper(f.Severity)+": "+f.Message)
	}
	return strings.Join(parts, "; ")
}

func formatSeconds(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}

func suiteSummaryText(suite SuiteResult) string {
	parts := make([]string, 0, 3)
	failedCases := 0
	for _, c := range suite.Cases {
		if !c.Passed {
			failedCases++
		}
	}
	if len(suite.Cases) > 0 {
		parts = append(parts, fmt.Sprintf("%d cases, %d failed", len(suite.Cases), failedCases))
	}
	if suite.Duration > 0 {
		parts = append(parts, suite.Duration.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " | ")
}

func caseSummaryText(c CaseResult) string {
	parts := make([]string, 0, 8)
	if c.Duration > 0 {
		parts = append(parts, "duration "+c.Duration.Round(time.Millisecond).String())
	}
	if c.Score > 0 {
		parts = append(parts, fmt.Sprintf("score %.2f", c.Score))
	}
	if c.LatencyP95 > 0 {
		parts = append(parts, "p95 "+c.LatencyP95.Round(time.Millisecond).String())
	}
	parts = append(parts, scalarDetailParts(c.Details)...)
	return strings.Join(parts, " | ")
}

func suiteMetaText(meta map[string]any) string {
	return strings.Join(scalarDetailParts(meta), " | ")
}

func scalarDetailParts(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if !isScalarReportValue(value) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, formatReportValue(values[key])))
	}
	return parts
}

func isScalarReportValue(value any) bool {
	switch value.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func formatReportValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float32:
		return fmt.Sprintf("%.2f", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprint(v)
	}
}

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

func writeFinding(b *strings.Builder, palette textPalette, indent int, finding Finding) {
	severity := strings.ToUpper(finding.Severity)
	fmt.Fprintf(b, "%s%s  %s: %s\n", strings.Repeat(" ", indent), palette.failureText(padRight("Finding", 7)), palette.failureText(severity), finding.Message)
}

func maxSuiteNameWidth(suites []SuiteResult) int {
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
