package devtools

import (
	"context"
	"fmt"
	"io"
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
	hygieneSection := mergeCodeGuardSections(
		"Hygiene",
		r.runCodeGuardDrySection(targets),
		r.runCodeGuardCleanCodeSection(targets),
	)

	sections := []codeGuardSectionResult{
		r.runCodeGuardGodFilesSection(ctx, opts, targets),
		r.runCodeGuardComplexitySection(ctx, opts, targets),
		r.runCodeGuardMaintainabilitySection(ctx, opts, targets),
		hygieneSection,
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
	result := codeGuardSectionResult{Name: "Large Files (High Cyclomatic)", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}
	prepared, failResult, ok := r.prepareCodeGuardGodFilesSection(ctx, opts, targets)
	if !ok {
		return failResult
	}

	for _, regression := range diffSCCRegressions(prepared.currentStats, prepared.baseStats, opts.MaxFileCodeLines) {
		path, message := splitCodeGuardPathMessage(regression)
		if _, ok := prepared.highComplexityPaths[path]; !ok {
			continue
		}
		if _, ok := prepared.allowlist[path]; ok {
			continue
		}
		result.Violations = append(result.Violations, codeGuardViolation{Path: path, Message: message})
	}
	if len(result.Violations) == 0 {
		if len(prepared.allowlist) > 0 {
			result.Note = "No new non-allowlisted file-size regressions detected."
		} else {
			result.Note = "No new maintainability regressions detected."
		}
		return result
	}
	sortCodeGuardViolations(result.Violations)
	result.Status = codeGuardStatusFail
	result.Note = fmt.Sprintf("files above %d code lines with new high-cyclomatic-complexity regressions", opts.MaxFileCodeLines)
	return result
}

type codeGuardGodFilesPrepared struct {
	allowlist           map[string]struct{}
	currentStats        map[string]sccFileReport
	baseStats           map[string]sccFileReport
	highComplexityPaths map[string]struct{}
}

func (r Runner) prepareCodeGuardGodFilesSection(ctx context.Context, opts CIOptions, targets []string) (codeGuardGodFilesPrepared, codeGuardSectionResult, bool) {
	allowlist, err := loadCodeGuardGodFilesAllowlist(r.WorkDir)
	if err != nil {
		return codeGuardGodFilesPrepared{}, codeGuardFailureResult("Large Files (High Cyclomatic)", "(config)", err), false
	}
	sccPath, err := r.ensureSCC(ctx, opts.SCCVersion)
	if err != nil {
		return codeGuardGodFilesPrepared{}, codeGuardFailureResult("Large Files (High Cyclomatic)", "(tooling)", err), false
	}
	currentStats, err := r.runSCCReport(ctx, sccPath, r.WorkDir, targets)
	if err != nil {
		return codeGuardGodFilesPrepared{}, codeGuardFailureResult("Large Files (High Cyclomatic)", "(runtime)", err), false
	}
	baseStats, err := r.loadBaseSCCStats(ctx, opts.BaseRef, sccPath, targets)
	if err != nil {
		return codeGuardGodFilesPrepared{}, codeGuardFailureResult("Large Files (High Cyclomatic)", "(baseline)", err), false
	}
	highComplexityPaths, err := r.loadGocycloRegressionPaths(ctx, opts, targets)
	if err != nil {
		return codeGuardGodFilesPrepared{}, codeGuardFailureResult("Large Files (High Cyclomatic)", "(complexity)", err), false
	}
	return codeGuardGodFilesPrepared{
		allowlist:           allowlist,
		currentStats:        currentStats,
		baseStats:           baseStats,
		highComplexityPaths: highComplexityPaths,
	}, codeGuardSectionResult{}, true
}

func (r Runner) loadGocycloRegressionPaths(ctx context.Context, opts CIOptions, targets []string) (map[string]struct{}, error) {
	gocycloPath, err := r.ensureGoTool(ctx, "gocyclo", "github.com/fzipp/gocyclo/cmd/gocyclo", opts.GocycloVersion)
	if err != nil {
		return nil, err
	}
	out, err := r.runOutputCommand(ctx, nil, gocycloPath, append([]string{"-over", strconv.Itoa(opts.MaxFunctionComplexity)}, targets...)...)
	if err != nil {
		return nil, err
	}
	currentFindings, err := parseGocycloFindings(out)
	if err != nil {
		return nil, err
	}
	baseFindings, err := r.loadBaseGocycloFindings(ctx, opts.BaseRef, gocycloPath, targets, opts.MaxFunctionComplexity)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]struct{})
	for _, regression := range diffGocycloRegressions(currentFindings, baseFindings) {
		path, _ := splitCodeGuardPathMessage(regression)
		paths[path] = struct{}{}
	}
	return paths, nil
}

func (r Runner) runCodeGuardComplexitySection(ctx context.Context, opts CIOptions, targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Cyclomatic Complexity", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}
	currentFindings, baseFindings, failResult, ok := r.prepareCodeGuardComplexitySection(ctx, opts, targets)
	if !ok {
		return failResult
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

func (r Runner) prepareCodeGuardComplexitySection(ctx context.Context, opts CIOptions, targets []string) (map[string]gocycloFinding, map[string]gocycloFinding, codeGuardSectionResult, bool) {
	gocycloPath, err := r.ensureGoTool(ctx, "gocyclo", "github.com/fzipp/gocyclo/cmd/gocyclo", opts.GocycloVersion)
	if err != nil {
		return nil, nil, codeGuardFailureResult("Cyclomatic Complexity", "(tooling)", err), false
	}
	out, err := r.runOutputCommand(ctx, nil, gocycloPath, append([]string{"-over", strconv.Itoa(opts.MaxFunctionComplexity)}, targets...)...)
	if err != nil {
		return nil, nil, codeGuardFailureResult("Cyclomatic Complexity", "(runtime)", err), false
	}
	currentFindings, err := parseGocycloFindings(out)
	if err != nil {
		return nil, nil, codeGuardFailureResult("Cyclomatic Complexity", "(parse)", err), false
	}
	baseFindings, err := r.loadBaseGocycloFindings(ctx, opts.BaseRef, gocycloPath, targets, opts.MaxFunctionComplexity)
	if err != nil {
		return nil, nil, codeGuardFailureResult("Cyclomatic Complexity", "(baseline)", err), false
	}
	return currentFindings, baseFindings, codeGuardSectionResult{}, true
}

func (r Runner) runCodeGuardMaintainabilitySection(ctx context.Context, opts CIOptions, targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Maintainability Lint", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}
	golangciLintPath, baseline, failResult, ok := r.prepareCodeGuardMaintainabilitySection(ctx, opts)
	if !ok {
		return failResult
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
		return codeGuardFailureResult("Maintainability Lint", "(runtime)", err)
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

func (r Runner) prepareCodeGuardMaintainabilitySection(ctx context.Context, opts CIOptions) (string, string, codeGuardSectionResult, bool) {
	golangciLintPath, err := r.ensureGolangCILint(ctx, opts.GolangCILintVersion)
	if err != nil {
		return "", "", codeGuardFailureResult("Maintainability Lint", "(tooling)", err), false
	}
	baseline, hasMergeBase, err := r.gitDiffBase(ctx, opts.BaseRef)
	if err != nil {
		return "", "", codeGuardFailureResult("Maintainability Lint", "(baseline)", err), false
	}
	label := "baseline"
	if !hasMergeBase {
		label = "fallback baseline"
	}
	if _, err := fmt.Fprintf(r.Stdout, "running golangci-lint against %s %s\n", label, baseline); err != nil {
		return "", "", codeGuardFailureResult("Maintainability Lint", "(runtime)", err), false
	}
	return golangciLintPath, baseline, codeGuardSectionResult{}, true
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

func codeGuardFailureResult(name, path string, err error) codeGuardSectionResult {
	return codeGuardSectionResult{
		Name:       name,
		Status:     codeGuardStatusFail,
		Note:       err.Error(),
		Violations: []codeGuardViolation{{Path: path, Message: err.Error()}},
	}
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
	defer func() {
		_ = f.Close()
	}()

	text, _ := renderCodeGuard(results)
	_ = writeCodeGuardSummaryBlock(f, strings.TrimPrefix(text, "\n"))
}

func writeCodeGuardSummaryBlock(w io.Writer, body string) error {
	lines := []string{"## CodeGuard", "", "```text", body, "```", ""}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
