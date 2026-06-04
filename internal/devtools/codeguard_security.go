package devtools

import (
	"context"
	"fmt"
	"strings"
)

func (r Runner) runCodeGuardGovulncheckSection(ctx context.Context, opts CIOptions) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Security: Go Vulnerabilities", Status: codeGuardStatusPass}

	govulncheckPath, err := r.ensureGoTool(ctx, "govulncheck", "golang.org/x/vuln/cmd/govulncheck", opts.GovulncheckVersion)
	if err != nil {
		result.Status = codeGuardStatusSkip
		result.Note = fmt.Sprintf("govulncheck unavailable: %v", err)
		return result
	}

	out, exitCode, err := r.runOutputCommandWithExitCode(ctx, nil, govulncheckPath, "./...")
	if err != nil {
		result.Status = codeGuardStatusSkip
		result.Note = err.Error()
		return result
	}
	if exitCode == 0 {
		result.Note = "No known Go vulnerabilities detected."
		return result
	}

	result.Status = codeGuardStatusWarn
	result.Note = "govulncheck reported findings"
	for _, line := range splitNonEmptyLines(firstLines(out, 40)) {
		result.Violations = append(result.Violations, codeGuardViolation{
			Path:    "(go)",
			Message: line,
		})
	}
	if len(result.Violations) == 0 {
		result.Violations = []codeGuardViolation{{Path: "(go)", Message: "govulncheck reported findings; see job logs"}}
	}
	return result
}

func (r Runner) runCodeGuardCriticalFilesSection(changedFiles []string) codeGuardSectionResult {
	return buildWarningSection(
		"Security: Critical Files",
		"critical cleanr files modified",
		filterMatching(changedFiles, ciCriticalFilesPattern),
	)
}

func (r Runner) runCodeGuardDangerousPatternsSection(diffText string) codeGuardSectionResult {
	return buildWarningSection(
		"Security: Dangerous Patterns",
		"potentially dangerous additions detected",
		filterMatching(splitNonEmptyLines(diffText), ciDangerousAddPattern),
	)
}

func (r Runner) runCodeGuardDependencyChangesSection(goModDiff string) codeGuardSectionResult {
	return buildWarningSection(
		"Security: Dependency Changes",
		"go.mod additions detected",
		filterMatching(splitNonEmptyLines(goModDiff), ciGoModAdditionPattern),
	)
}

func buildWarningSection(name, note string, lines []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: name, Status: codeGuardStatusPass}
	if len(lines) == 0 {
		result.Note = "No findings."
		return result
	}

	result.Status = codeGuardStatusWarn
	result.Note = note
	for _, line := range lines[:min(len(lines), 40)] {
		path, message := splitCodeGuardPathMessage(line)
		if path == "(repo)" && strings.TrimSpace(message) == strings.TrimSpace(line) {
			message = line
		}
		result.Violations = append(result.Violations, codeGuardViolation{
			Path:    path,
			Message: message,
		})
	}
	return result
}
