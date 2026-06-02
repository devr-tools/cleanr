package devtools

import (
	"fmt"
	"strings"
)

func renderCodeGuard(results []codeGuardSectionResult) (string, bool) {
	blockingFailed := false
	for _, result := range results {
		if result.blockingFailed() {
			blockingFailed = true
			break
		}
	}

	parts := []string{"", "Code Guard", renderCodeGuardTable(results)}
	if details := renderCodeGuardDetails(results, 50); details != "" {
		parts = append(parts, "", details)
	}

	var warned []string
	var skipped []string
	for _, result := range results {
		switch result.Status {
		case codeGuardStatusWarn:
			warned = append(warned, result.Name)
		case codeGuardStatusSkip:
			skipped = append(skipped, result.Name)
		}
	}
	if len(warned) > 0 {
		parts = append(parts, "", "Warnings (non-blocking): "+strings.Join(warned, ", "))
	}
	if len(skipped) > 0 {
		parts = append(parts, "Skipped: "+strings.Join(skipped, ", "))
	}

	resultText := "PASS"
	if blockingFailed {
		resultText = "FAIL"
	}
	parts = append(parts, "", "RESULT: "+resultText)
	return strings.Join(parts, "\n"), !blockingFailed
}

func renderCodeGuardTable(results []codeGuardSectionResult) string {
	headers := []string{"Check", "Status", "Files"}
	rows := make([][]string, 0, len(results))
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2])}
	for _, result := range results {
		row := []string{result.Name, string(result.Status), fmt.Sprintf("%d", result.fileCount())}
		rows = append(rows, row)
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	lines := []string{
		renderCodeGuardDivider(widths),
		renderCodeGuardRow(headers, widths),
		renderCodeGuardDivider(widths),
	}
	for _, row := range rows {
		lines = append(lines, renderCodeGuardRow(row, widths))
	}
	lines = append(lines, renderCodeGuardDivider(widths))
	return strings.Join(lines, "\n")
}

func renderCodeGuardDetails(results []codeGuardSectionResult, maxPerSection int) string {
	var blocks []string
	for _, result := range results {
		if len(result.Violations) == 0 {
			continue
		}
		if result.Status == codeGuardStatusPass || result.Status == codeGuardStatusSkip {
			continue
		}
		header := fmt.Sprintf("%s [%s]", result.Name, result.Status)
		if result.Note != "" {
			header += " - " + result.Note
		}

		lines := []string{header}
		limit := len(result.Violations)
		if limit > maxPerSection {
			limit = maxPerSection
		}
		for _, violation := range result.Violations[:limit] {
			lines = append(lines, fmt.Sprintf("    %s: %s", violation.Path, violation.Message))
		}
		if remaining := len(result.Violations) - limit; remaining > 0 {
			lines = append(lines, fmt.Sprintf("    ... and %d more", remaining))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func renderCodeGuardRow(cells []string, widths []int) string {
	padded := make([]string, 0, len(cells))
	for i, cell := range cells {
		padded = append(padded, " "+cell+strings.Repeat(" ", widths[i]-len(cell))+" ")
	}
	return "|" + strings.Join(padded, "|") + "|"
}

func renderCodeGuardDivider(widths []int) string {
	segments := make([]string, 0, len(widths))
	for _, width := range widths {
		segments = append(segments, strings.Repeat("-", width+2))
	}
	return "+" + strings.Join(segments, "+") + "+"
}
