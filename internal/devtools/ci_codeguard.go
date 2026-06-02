package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (r Runner) CodeGuard(ctx context.Context, opts CIOptions) error {
	resolved, err := r.resolveCIOptions(ctx, opts)
	if err != nil {
		return err
	}
	return r.runCICodeGuard(ctx, resolved)
}

func (r Runner) runCICodeGuard(ctx context.Context, opts CIOptions) error {
	changedFiles, err := r.gitChangedFiles(ctx, opts.BaseRef)
	if err != nil {
		return err
	}
	diffText, err := r.gitDiff(ctx, opts.BaseRef)
	if err != nil {
		return err
	}
	goModDiff, err := r.runOutputCommand(ctx, nil, "git", "diff", opts.BaseRef, "--", "go.mod")
	if err != nil {
		return err
	}
	targets := filterCICodeGuardTargets(changedFiles)

	sections := []codeGuardSectionResult{
		r.runCodeGuardGodFilesSection(ctx, opts, targets),
		r.runCodeGuardComplexitySection(ctx, opts, targets),
		r.runCodeGuardMaintainabilitySection(ctx, opts, targets),
		r.runCodeGuardDrySection(targets),
		r.runCodeGuardCleanCodeSection(targets),
		r.runCodeGuardPrinciplesSection(targets),
		r.runCodeGuardGovulncheckSection(ctx, opts),
		r.runCodeGuardCriticalFilesSection(changedFiles),
		r.runCodeGuardDangerousPatternsSection(diffText),
		r.runCodeGuardDependencyChangesSection(goModDiff),
	}

	text, ok := renderCodeGuard(sections)
	if _, err := fmt.Fprintln(r.Stdout, text); err != nil {
		return err
	}
	r.writeCodeGuardStepSummary(sections)
	if !ok {
		return fmt.Errorf("codeguard failed")
	}
	return nil
}

func (r Runner) runCodeGuardGodFilesSection(ctx context.Context, opts CIOptions, targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "God Files", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}
	allowlist, err := loadCodeGuardGodFilesAllowlist(r.WorkDir)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(config)", Message: err.Error()}}
		return result
	}

	sccPath, err := r.ensureSCC(ctx, opts.SCCVersion)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(tooling)", Message: err.Error()}}
		return result
	}
	currentStats, err := r.runSCCReport(ctx, sccPath, r.WorkDir, targets)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(runtime)", Message: err.Error()}}
		return result
	}
	baseStats, err := r.loadBaseSCCStats(ctx, opts.BaseRef, sccPath, targets)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(baseline)", Message: err.Error()}}
		return result
	}

	for _, regression := range diffSCCRegressions(currentStats, baseStats, opts.MaxFileCodeLines) {
		path, message := splitCodeGuardPathMessage(regression)
		if _, ok := allowlist[path]; ok {
			continue
		}
		result.Violations = append(result.Violations, codeGuardViolation{Path: path, Message: message})
	}
	if len(result.Violations) == 0 {
		if len(allowlist) > 0 {
			result.Note = "No new non-allowlisted file-size regressions detected."
		} else {
			result.Note = "No new maintainability regressions detected."
		}
		return result
	}
	sortCodeGuardViolations(result.Violations)
	result.Status = codeGuardStatusFail
	result.Note = fmt.Sprintf("files above %d code lines", opts.MaxFileCodeLines)
	return result
}

func (r Runner) runCodeGuardComplexitySection(ctx context.Context, opts CIOptions, targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Cyclomatic Complexity", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}

	gocycloPath, err := r.ensureGoTool(ctx, "gocyclo", "github.com/fzipp/gocyclo/cmd/gocyclo", opts.GocycloVersion)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(tooling)", Message: err.Error()}}
		return result
	}
	out, err := r.runOutputCommand(ctx, nil, gocycloPath, append([]string{"-over", strconv.Itoa(opts.MaxFunctionComplexity)}, targets...)...)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(runtime)", Message: err.Error()}}
		return result
	}
	currentFindings, err := parseGocycloFindings(out)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(parse)", Message: err.Error()}}
		return result
	}
	baseFindings, err := r.loadBaseGocycloFindings(ctx, opts.BaseRef, gocycloPath, targets, opts.MaxFunctionComplexity)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(baseline)", Message: err.Error()}}
		return result
	}

	for _, regression := range diffGocycloRegressions(currentFindings, baseFindings) {
		path, message := splitCodeGuardPathMessage(regression)
		result.Violations = append(result.Violations, codeGuardViolation{Path: path, Message: message})
	}
	if len(result.Violations) == 0 {
		result.Note = "No new maintainability regressions detected."
		return result
	}
	sortCodeGuardViolations(result.Violations)
	result.Status = codeGuardStatusFail
	result.Note = fmt.Sprintf("functions above complexity %d", opts.MaxFunctionComplexity)
	return result
}

func (r Runner) runCodeGuardMaintainabilitySection(ctx context.Context, opts CIOptions, targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Maintainability Lint", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}

	golangciLintPath, err := r.ensureGolangCILint(ctx, opts.GolangCILintVersion)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(tooling)", Message: err.Error()}}
		return result
	}
	baseline, hasMergeBase, err := r.gitDiffBase(ctx, opts.BaseRef)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(baseline)", Message: err.Error()}}
		return result
	}
	label := "baseline"
	if !hasMergeBase {
		label = "fallback baseline"
	}
	if _, err := fmt.Fprintf(r.Stdout, "running golangci-lint against %s %s\n", label, baseline); err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(runtime)", Message: err.Error()}}
		return result
	}

	out, exitCode, err := r.runOutputCommandWithExitCode(
		ctx,
		nil,
		golangciLintPath,
		"run",
		"--config",
		".golangci.yml",
		"--new-from-rev",
		baseline,
		"--whole-files",
		"./...",
	)
	if err != nil {
		result.Status = codeGuardStatusFail
		result.Note = err.Error()
		result.Violations = []codeGuardViolation{{Path: "(runtime)", Message: err.Error()}}
		return result
	}
	if exitCode == 0 {
		result.Note = "No new maintainability regressions detected."
		return result
	}
	if exitCode != 1 {
		message := fmt.Sprintf("golangci-lint exited with code %d", exitCode)
		result.Status = codeGuardStatusFail
		result.Note = message
		result.Violations = []codeGuardViolation{{Path: "(runtime)", Message: message}}
		return result
	}

	violations := parseGolangCILintViolations(out)
	if len(violations) == 0 {
		violations = []codeGuardViolation{{Path: "(repo)", Message: "golangci-lint reported new findings; see job logs"}}
	}
	sortCodeGuardViolations(violations)
	result.Status = codeGuardStatusFail
	result.Note = "new golangci-lint findings"
	result.Violations = violations
	return result
}

func parseGolangCILintViolations(raw string) []codeGuardViolation {
	var violations []codeGuardViolation
	for _, line := range splitNonEmptyLines(raw) {
		path, message := splitCodeGuardPathMessage(line)
		violations = append(violations, codeGuardViolation{Path: path, Message: message})
	}
	return violations
}

func splitCodeGuardPathMessage(text string) (string, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "(unknown)", ""
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "(unknown)", trimmed
	}
	first := fields[0]
	if looksLikePath(first) {
		return trimPathToken(first), strings.TrimSpace(strings.TrimPrefix(trimmed, first))
	}
	if idx := strings.Index(trimmed, ": "); idx > 0 {
		candidate := trimmed[:idx]
		if looksLikePath(candidate) {
			return trimPathToken(candidate), strings.TrimSpace(trimmed[idx+2:])
		}
	}
	return "(repo)", trimmed
}

func looksLikePath(value string) bool {
	candidate := trimPathToken(value)
	return strings.Contains(candidate, "/") || strings.HasSuffix(candidate, ".go")
}

func trimPathToken(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ":")
	parts := strings.Split(trimmed, ":")
	if len(parts) == 1 {
		return filepath.ToSlash(trimmed)
	}

	end := len(parts)
	for end > 1 {
		if _, err := strconv.Atoi(parts[end-1]); err == nil {
			end--
			continue
		}
		break
	}
	return filepath.ToSlash(strings.Join(parts[:end], ":"))
}

func (r Runner) writeCodeGuardStepSummary(results []codeGuardSectionResult) {
	summaryPath := strings.TrimSpace(os.Getenv("GITHUB_STEP_SUMMARY"))
	if summaryPath == "" {
		return
	}
	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	text, _ := renderCodeGuard(results)
	_, _ = fmt.Fprintln(f, "## CodeGuard")
	_, _ = fmt.Fprintln(f)
	_, _ = fmt.Fprintln(f, "```text")
	_, _ = fmt.Fprintln(f, strings.TrimPrefix(text, "\n"))
	_, _ = fmt.Fprintln(f, "```")
	_, _ = fmt.Fprintln(f)
}
