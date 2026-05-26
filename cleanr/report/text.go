package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	imgpkg "github.com/devr-tools/cleanr/img"
)

func renderText(report core.Report, palette textPalette) string {
	var b strings.Builder
	writeReportSummary(&b, palette, report)
	writeReportOverview(&b, palette, report)
	writeReportDetails(&b, palette, report)
	writeReportTrends(&b, palette, report)
	writeReportTrendGates(&b, palette, report)
	writeReportIntegrations(&b, palette, report)
	writeReportRecommendations(&b, palette, report)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeReportSummary(b *strings.Builder, palette textPalette, report core.Report) {
	status := "PASS"
	if !report.Passed {
		status = "FAIL"
	}
	if banner := renderBanner(palette); banner != "" {
		fmt.Fprintf(b, "%s\n\n", banner)
	}
	fmt.Fprintf(b, "%s\n", palette.accent("Report Summary"))
	fmt.Fprintf(b, "%s\n", palette.accent(strings.Repeat("=", 48)))
	writeKeyValue(b, palette, "Status", palette.status(report.Passed, status))
	writeKeyValue(b, palette, "Target", report.Name)
	if !report.GeneratedAt.IsZero() {
		writeKeyValue(b, palette, "Generated", report.GeneratedAt.Format(time.RFC3339))
	}
	writeKeyValue(b, palette, "Duration", report.Duration.Round(time.Millisecond).String())
	writeKeyValue(b, palette, "Suites", fmt.Sprintf("%d total | %s", report.TotalSuites, palette.failedCount(report.FailedSuites)))
	writeKeyValue(b, palette, "Cases", fmt.Sprintf("%d total | %s", report.TotalCases, palette.failedCount(report.FailedCases)))
}

func writeReportOverview(b *strings.Builder, palette textPalette, report core.Report) {
	writeSectionHeader(b, palette, "Overview")
	suiteWidth := maxSuiteNameWidth(report.Suites)
	for _, suite := range report.Suites {
		fmt.Fprintf(b, "%s %-*s  %s\n", palette.badge(suite.Passed), suiteWidth, suite.Name, suiteSummaryText(suite))
	}
}

func writeReportDetails(b *strings.Builder, palette textPalette, report core.Report) {
	writeSectionHeader(b, palette, "Details")
	for i, suite := range report.Suites {
		if i > 0 {
			fmt.Fprintln(b)
		}
		writeSuiteDetails(b, palette, suite)
	}
}

func writeSuiteDetails(b *strings.Builder, palette textPalette, suite core.SuiteResult) {
	fmt.Fprintf(b, "%s %s\n", suite.Name, palette.badge(suite.Passed))
	if summary := suiteSummaryText(suite); summary != "" {
		writeIndentedValue(b, palette, 2, "Summary", summary)
	}
	for _, c := range suite.Cases {
		writeCaseDetails(b, palette, c)
	}
	for _, f := range suite.Findings {
		writeFinding(b, palette, 2, f)
	}
	if meta := suiteMetaText(suite.Meta); meta != "" {
		writeIndentedValue(b, palette, 2, "Meta", meta)
	}
}

func writeCaseDetails(b *strings.Builder, palette textPalette, c core.CaseResult) {
	fmt.Fprintf(b, "  - %s %s\n", c.Name, palette.badge(c.Passed))
	if summary := caseSummaryText(c); summary != "" {
		writeIndentedValue(b, palette, 4, "Metrics", summary)
	}
	for _, f := range c.Findings {
		writeFinding(b, palette, 4, f)
	}
	for _, detail := range structuredDetailParts(c.Details) {
		writeIndentedValue(b, palette, 4, detail.Key, detail.Value)
	}
}

func writeReportTrends(b *strings.Builder, palette textPalette, report core.Report) {
	if report.Trend == nil {
		return
	}
	writeSectionHeader(b, palette, "Trends")
	if report.Trend.Baseline {
		writeIndentedValue(b, palette, 2, "Baseline", "captured first history point for this target")
		return
	}
	writeIndentedValue(b, palette, 2, "Compared", trendComparedText(*report.Trend))
	writeIndentedValue(b, palette, 2, "Summary", trendSummaryText(report.Trend.Summary))
	writeTrendBuildDiff(b, palette, report.Trend.BuildDiff)
	for _, suiteTrend := range report.Trend.Suites {
		if shouldSkipSuiteTrend(suiteTrend) {
			continue
		}
		writeIndentedValue(b, palette, 2, suiteTrend.Name, suiteTrendText(suiteTrend))
	}
}

func writeTrendBuildDiff(b *strings.Builder, palette textPalette, diff *core.BuildDiff) {
	if diff == nil {
		return
	}
	if header := buildDiffHeaderText(*diff); header != "" {
		writeIndentedValue(b, palette, 2, "BuildDiff", header)
	}
	for _, change := range diff.ScenarioChanges {
		writeIndentedValue(b, palette, 2, "Scenario", scenarioBuildDiffText(change))
	}
}

func shouldSkipSuiteTrend(suiteTrend core.SuiteTrend) bool {
	return suiteTrend.Status == "unchanged" && suiteTrend.Drift == nil && suiteTrend.FailedCasesDelta == 0 && suiteTrend.ScoreDelta == 0
}

func writeReportTrendGates(b *strings.Builder, palette textPalette, report core.Report) {
	if report.TrendGate == nil {
		return
	}
	writeSectionHeader(b, palette, "Trend Gates")
	writeIndentedValue(b, palette, 2, "Status", trendGateStatusText(*report.TrendGate))
	if report.TrendGate.RequiredWindow > 0 {
		writeIndentedValue(b, palette, 2, "Window", fmt.Sprintf("%d required | %d available", report.TrendGate.RequiredWindow, report.TrendGate.AvailableWindow))
	}
	for _, finding := range report.TrendGate.Findings {
		writeFinding(b, palette, 2, finding)
	}
}

func trendGateStatusText(gate core.TrendGateReport) string {
	if !gate.Evaluated {
		return "SKIPPED"
	}
	if gate.Passed {
		return "PASS"
	}
	return "FAIL"
}

func writeReportIntegrations(b *strings.Builder, palette textPalette, report core.Report) {
	if report.Integrations == nil {
		return
	}
	writeSectionHeader(b, palette, "Integrations")
	writeIndentedValue(b, palette, 2, "Contract", "local gate remains blocking | remote integrations are best-effort")
	writeTrendSourceIntegrations(b, palette, report.Integrations.TrendSources)
	writeResultSinkIntegrations(b, palette, report.Integrations.ResultSinks)
	writeSummaryIntegrations(b, palette, report.Integrations.Summaries)
}

func writeTrendSourceIntegrations(b *strings.Builder, palette textPalette, sources []core.ExternalTrendReport) {
	for _, source := range sources {
		writeIndentedValue(b, palette, 2, source.Name, trendSourceIntegrationText(source))
	}
}

func trendSourceIntegrationText(source core.ExternalTrendReport) string {
	status := source.Status
	if strings.TrimSpace(status) == "" {
		status = "unknown"
	}
	text := strings.ToUpper(status)
	if source.Summary != nil {
		text += " | " + trendSummaryText(*source.Summary)
	}
	if source.ViewURL != "" {
		text += " | view=" + source.ViewURL
	}
	if source.Message != "" && source.Status != "compared" {
		text += " | " + source.Message
	}
	return text
}

func writeResultSinkIntegrations(b *strings.Builder, palette textPalette, sinks []core.ResultSinkReport) {
	for _, sink := range sinks {
		writeIndentedValue(b, palette, 2, sink.Name, resultSinkIntegrationText(sink))
	}
}

func resultSinkIntegrationText(sink core.ResultSinkReport) string {
	status := "FAILED"
	if sink.Published {
		status = "PUBLISHED"
	}
	text := status
	if sink.RunURL != "" {
		text += " | view=" + sink.RunURL
	}
	if sink.Message != "" {
		text += " | " + sink.Message
	}
	return text
}

func writeSummaryIntegrations(b *strings.Builder, palette textPalette, summaries []core.SummaryArtifactReport) {
	for _, summary := range summaries {
		writeIndentedValue(b, palette, 2, summary.Name, summaryIntegrationText(summary))
	}
}

func summaryIntegrationText(summary core.SummaryArtifactReport) string {
	status := "FAILED"
	if summary.Written {
		status = "WRITTEN"
	}
	text := status
	if summary.Output != "" {
		text += " | output=" + summary.Output
	}
	if summary.Message != "" {
		text += " | " + summary.Message
	}
	return text
}

func writeReportRecommendations(b *strings.Builder, palette textPalette, report core.Report) {
	if len(report.Recommendations) == 0 {
		return
	}
	writeSectionHeader(b, palette, "Recommendations")
	for _, rec := range report.Recommendations {
		fmt.Fprintf(b, "  - %s\n", rec)
	}
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
