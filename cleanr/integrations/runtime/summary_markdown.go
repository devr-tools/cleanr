package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func renderMarkdownSummary(report core.Report) string {
	var b strings.Builder
	writeMarkdownSummaryHeader(&b, report)
	writeMarkdownFailureSummary(&b, report)
	writeMarkdownRemoteComparisons(&b, report)
	writeMarkdownRemoteViews(&b, report)
	writeMarkdownRecommendations(&b, report)
	return b.String()
}

func writeMarkdownSummaryHeader(b *strings.Builder, report core.Report) {
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(b, "# cleanr Release Summary\n\n")
	fmt.Fprintf(b, "- Local gate: `%s` (blocking)\n", status)
	fmt.Fprintf(b, "- Target: `%s`\n", report.Name)
	if build := buildID(report.Metadata); build != "" {
		fmt.Fprintf(b, "- Build: `%s`\n", build)
	}
	if !report.GeneratedAt.IsZero() {
		fmt.Fprintf(b, "- Generated: `%s`\n", report.GeneratedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(b, "- Failed suites: `%d`\n", report.FailedSuites)
	fmt.Fprintf(b, "- Failed cases: `%d`\n", report.FailedCases)
	if report.Trend != nil && !report.Trend.Baseline {
		fmt.Fprintf(b, "- Local trend: `%+d suites`, `%+d cases`, `%s duration`\n", report.Trend.Summary.FailedSuitesDelta, report.Trend.Summary.FailedCasesDelta, report.Trend.Summary.DurationDelta.Round(time.Millisecond))
	}
	if report.TrendGate != nil {
		fmt.Fprintf(b, "- Trend gates: `%s`\n", markdownTrendGateStatus(*report.TrendGate))
	}
}

func markdownTrendGateStatus(gate core.TrendGateReport) string {
	if !gate.Evaluated {
		return "SKIPPED"
	}
	if gate.Passed {
		return "PASS"
	}
	return "FAIL"
}

func writeMarkdownFailureSummary(b *strings.Builder, report core.Report) {
	failures := failureSummary(report)
	if len(failures) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Local Failures\n\n")
	for _, line := range failures {
		fmt.Fprintf(b, "- %s\n", line)
	}
}

func writeMarkdownRemoteComparisons(b *strings.Builder, report core.Report) {
	if report.Integrations == nil || len(report.Integrations.TrendSources) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Remote Comparisons\n\n")
	for _, source := range report.Integrations.TrendSources {
		fmt.Fprintf(b, "- %s\n", markdownRemoteComparisonLine(source))
	}
}

func markdownRemoteComparisonLine(source core.ExternalTrendReport) string {
	line := fmt.Sprintf("`%s`: `%s`", source.Name, strings.ToUpper(emptyStatus(source.Status)))
	if source.Summary != nil {
		line += fmt.Sprintf(" against `%s` with `%+d` failed-case delta", emptyValue(source.LatestBuildID), source.Summary.FailedCasesDelta)
	}
	if source.ViewURL != "" {
		line += fmt.Sprintf(" ([view](%s))", source.ViewURL)
	}
	if source.Message != "" && source.Status != "compared" {
		line += ": " + source.Message
	}
	return line
}

func writeMarkdownRemoteViews(b *strings.Builder, report core.Report) {
	if report.Integrations == nil {
		return
	}
	links := remoteLinks(report.Integrations.ResultSinks)
	if len(links) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Remote Views\n\n")
	for _, line := range links {
		fmt.Fprintf(b, "- %s\n", line)
	}
}

func writeMarkdownRecommendations(b *strings.Builder, report core.Report) {
	if len(report.Recommendations) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Recommendations\n\n")
	for _, rec := range report.Recommendations {
		fmt.Fprintf(b, "- %s\n", rec)
	}
}

func failureSummary(report core.Report) []string {
	lines := make([]string, 0)
	for _, suite := range report.Suites {
		for _, finding := range suite.Findings {
			lines = append(lines, fmt.Sprintf("%s: %s", suite.Name, finding.Message))
			if len(lines) == 8 {
				return lines
			}
		}
		for _, c := range suite.Cases {
			if c.Passed && len(c.Findings) == 0 {
				continue
			}
			line := fmt.Sprintf("%s/%s", suite.Name, c.Name)
			if len(c.Findings) > 0 {
				line += ": " + c.Findings[0].Message
			}
			lines = append(lines, line)
			if len(lines) == 8 {
				return lines
			}
		}
	}
	return lines
}

func remoteLinks(items []core.ResultSinkReport) []string {
	out := make([]string, 0)
	for _, item := range items {
		if strings.TrimSpace(item.RunURL) == "" {
			continue
		}
		out = append(out, fmt.Sprintf("`%s`: %s", item.Name, item.RunURL))
	}
	return out
}
