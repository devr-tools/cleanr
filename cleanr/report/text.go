package report

import (
	"fmt"
	"strings"
	"time"

	"cleanr/cleanr/core"
	imgpkg "cleanr/img"
)

func renderText(report core.Report, palette textPalette) string {
	var b strings.Builder
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	if banner := renderBanner(palette); banner != "" {
		fmt.Fprintf(&b, "%s\n\n", banner)
	}
	fmt.Fprintf(&b, "%s\n", palette.accent("Report Summary"))
	fmt.Fprintf(&b, "%s\n", palette.accent(strings.Repeat("=", 48)))
	writeKeyValue(&b, palette, "Status", palette.status(report.Passed, status))
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
			for _, detail := range structuredDetailParts(c.Details) {
				writeIndentedValue(&b, palette, 4, detail.Key, detail.Value)
			}
		}
		for _, f := range suite.Findings {
			writeFinding(&b, palette, 2, f)
		}
		if meta := suiteMetaText(suite.Meta); meta != "" {
			writeIndentedValue(&b, palette, 2, "Meta", meta)
		}
	}
	if report.Trend != nil {
		writeSectionHeader(&b, palette, "Trends")
		if report.Trend.Baseline {
			writeIndentedValue(&b, palette, 2, "Baseline", "captured first history point for this target")
		} else {
			writeIndentedValue(&b, palette, 2, "Compared", trendComparedText(*report.Trend))
			writeIndentedValue(&b, palette, 2, "Summary", trendSummaryText(report.Trend.Summary))
			if report.Trend.BuildDiff != nil {
				if header := buildDiffHeaderText(*report.Trend.BuildDiff); header != "" {
					writeIndentedValue(&b, palette, 2, "BuildDiff", header)
				}
				for _, change := range report.Trend.BuildDiff.ScenarioChanges {
					writeIndentedValue(&b, palette, 2, "Scenario", scenarioBuildDiffText(change))
				}
			}
			for _, suiteTrend := range report.Trend.Suites {
				if suiteTrend.Status == "unchanged" && suiteTrend.Drift == nil && suiteTrend.FailedCasesDelta == 0 && suiteTrend.ScoreDelta == 0 {
					continue
				}
				writeIndentedValue(&b, palette, 2, suiteTrend.Name, suiteTrendText(suiteTrend))
			}
		}
	}
	if report.TrendGate != nil {
		writeSectionHeader(&b, palette, "Trend Gates")
		status := "SKIPPED"
		if report.TrendGate.Evaluated {
			if report.TrendGate.Passed {
				status = "PASS"
			} else {
				status = "FAIL"
			}
		}
		writeIndentedValue(&b, palette, 2, "Status", status)
		if report.TrendGate.RequiredWindow > 0 {
			writeIndentedValue(&b, palette, 2, "Window", fmt.Sprintf("%d required | %d available", report.TrendGate.RequiredWindow, report.TrendGate.AvailableWindow))
		}
		for _, finding := range report.TrendGate.Findings {
			writeFinding(&b, palette, 2, finding)
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

func renderBanner(palette textPalette) string {
	banner := imgpkg.Banner()
	if banner == "" {
		return ""
	}
	if !palette.color {
		return banner
	}
	lines := strings.Split(banner, "\n")
	for i, line := range lines {
		lines[i] = palette.accent(line)
	}
	return strings.Join(lines, "\n")
}

func buildDiffHeaderText(diff core.BuildDiff) string {
	parts := make([]string, 0, 2)
	if diff.TargetTypeBefore != "" || diff.TargetTypeAfter != "" {
		parts = append(parts, fmt.Sprintf("target_type=%s -> %s", emptyValue(diff.TargetTypeBefore), emptyValue(diff.TargetTypeAfter)))
	}
	if diff.ModelBefore != "" || diff.ModelAfter != "" {
		parts = append(parts, fmt.Sprintf("model=%s -> %s", emptyValue(diff.ModelBefore), emptyValue(diff.ModelAfter)))
	}
	return strings.Join(parts, " | ")
}

func scenarioBuildDiffText(change core.ScenarioDiff) string {
	parts := []string{change.Name, change.Status}
	if change.SystemChanged {
		parts = append(parts, "system")
	}
	if change.InputChanged {
		parts = append(parts, "input")
	}
	if change.ContextChanged {
		parts = append(parts, "context")
	}
	if change.MemoryReplayChanged {
		parts = append(parts, "memory_replay")
	}
	if change.TagsChanged {
		parts = append(parts, "tags")
	}
	return strings.Join(parts, " | ")
}

func emptyValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<unset>"
	}
	return value
}
