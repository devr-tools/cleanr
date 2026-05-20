package cleanr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

func WriteReport(w io.Writer, report Report, format string) error {
	switch strings.ToLower(format) {
	case "", "text":
		_, err := fmt.Fprint(w, TextReport(report))
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
	var b strings.Builder
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "cleanr %s\n", status)
	fmt.Fprintf(&b, "target: %s\n", report.Name)
	if !report.GeneratedAt.IsZero() {
		fmt.Fprintf(&b, "generated: %s\n", report.GeneratedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "duration: %s\n", report.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "summary: %d/%d suites failed, %d/%d cases failed\n", report.FailedSuites, report.TotalSuites, report.FailedCases, report.TotalCases)

	fmt.Fprintf(&b, "\nsuite summary:\n")
	for _, suite := range report.Suites {
		suiteStatus := "PASS"
		if !suite.Passed {
			suiteStatus = "FAIL"
		}
		fmt.Fprintf(&b, "  %s  %s", suiteStatus, suite.Name)
		if summary := suiteSummaryText(suite); summary != "" {
			fmt.Fprintf(&b, "  %s", summary)
		}
		fmt.Fprintf(&b, "\n")
		for _, c := range suite.Cases {
			caseStatus := "PASS"
			if !c.Passed {
				caseStatus = "FAIL"
			}
			fmt.Fprintf(&b, "    - %s [%s]", c.Name, caseStatus)
			if summary := caseSummaryText(c); summary != "" {
				fmt.Fprintf(&b, "  %s", summary)
			}
			fmt.Fprintf(&b, "\n")
			for _, f := range c.Findings {
				fmt.Fprintf(&b, "      %s: %s\n", strings.ToUpper(f.Severity), f.Message)
			}
		}
		for _, f := range suite.Findings {
			fmt.Fprintf(&b, "    %s: %s\n", strings.ToUpper(f.Severity), f.Message)
		}
		if meta := suiteMetaText(suite.Meta); meta != "" {
			fmt.Fprintf(&b, "    meta: %s\n", meta)
		}
	}
	if len(report.Recommendations) > 0 {
		fmt.Fprintf(&b, "\nrecommendations:\n")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(&b, "  - %s\n", rec)
		}
	}
	return b.String()
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
