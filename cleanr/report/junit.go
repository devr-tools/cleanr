package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

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

func writeJUnit(w io.Writer, report core.Report) error {
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

func findingsText(findings []core.Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, strings.ToUpper(f.Severity)+": "+f.Message)
	}
	return strings.Join(parts, "; ")
}

func formatSeconds(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}
