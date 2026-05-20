package cleanr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
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
	fmt.Fprintf(&b, "duration: %s\n", report.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "suites: %d total, %d failed\n", report.TotalSuites, report.FailedSuites)
	fmt.Fprintf(&b, "cases: %d total, %d failed\n", report.TotalCases, report.FailedCases)
	for _, suite := range report.Suites {
		suiteStatus := "PASS"
		if !suite.Passed {
			suiteStatus = "FAIL"
		}
		fmt.Fprintf(&b, "\n[%s] %s\n", suiteStatus, suite.Name)
		for _, c := range suite.Cases {
			caseStatus := "PASS"
			if !c.Passed {
				caseStatus = "FAIL"
			}
			fmt.Fprintf(&b, "  - %s (%s)\n", c.Name, caseStatus)
			for _, f := range c.Findings {
				fmt.Fprintf(&b, "    %s: %s\n", strings.ToUpper(f.Severity), f.Message)
			}
		}
		for _, f := range suite.Findings {
			fmt.Fprintf(&b, "  %s: %s\n", strings.ToUpper(f.Severity), f.Message)
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
