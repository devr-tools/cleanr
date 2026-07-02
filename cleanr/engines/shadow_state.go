package engines

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type ShadowStateEngine struct{}

func (ShadowStateEngine) Name() string { return "shadow-state" }

func (ShadowStateEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	roots, err := normalizeObservedPaths(runCtx.Config.Suites.ShadowState.Roots)
	if err != nil {
		return shadowStateSetupFailure(fmt.Errorf("normalize shadow-state roots: %w", err))
	}
	allowed, err := normalizeObservedPaths(runCtx.Config.Suites.ShadowState.AllowedWritePaths)
	if err != nil {
		return shadowStateSetupFailure(fmt.Errorf("normalize shadow-state allowed_write_paths: %w", err))
	}

	// shadow-state captures filesystem state around each invoke, so it stays
	// serial to avoid interleaving observations across concurrent scenarios.
	cases := make([]core.CaseResult, 0, len(runCtx.Config.Scenarios))
	for _, scenario := range runCtx.Config.Scenarios {
		if ctx.Err() != nil {
			break
		}
		cases = append(cases, runShadowStateScenario(ctx, runCtx, roots, allowed, scenario))
	}

	return core.SuiteResult{Name: "shadow-state", Passed: allPassed(cases), Cases: cases}
}

func runShadowStateScenario(ctx context.Context, runCtx *core.RunContext, roots, allowed []string, scenario core.Scenario) core.CaseResult {
	start := time.Now()
	findings := make([]core.Finding, 0)
	expected, err := normalizeExpectedMutations(scenario.ExpectedMutations)
	if err != nil {
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("normalize expected mutations: %v", err)})
	}
	before, err := captureObservedFiles(roots)
	if err != nil {
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("capture pre-run file state: %v", err)})
	}
	resp := runCtx.Target.Invoke(ctx, scenarioRequest(scenario, runCtx.Config.Target.Timeout()))
	findings = append(findings, responseFindings(resp, nil)...)
	after, err := captureObservedFiles(roots)
	if err != nil {
		findings = append(findings, core.Finding{Severity: "critical", Message: fmt.Sprintf("capture post-run file state: %v", err)})
	}
	changes := diffObservedFiles(before, after)
	approved, unexpected, approvedObserved := classifyObservedChanges(changes, allowed)
	findings = append(findings, unexpectedObservedChangeFindings(unexpected)...)
	matchedExpected, missingExpected, undeclaredApproved := matchExpectedMutations(expected, approvedObserved)
	findings = append(findings, expectedMutationFindings(missingExpected, undeclaredApproved)...)
	findings = append(findings, verifyExpectedMutationContent(matchedExpected)...)
	details := shadowStateDetails(resp, roots, changes, approved, unexpected, expected, matchedExpected, missingExpected, undeclaredApproved)
	return core.CaseResult{
		Name:     scenario.Name,
		Passed:   len(findings) == 0,
		Duration: time.Since(start),
		Findings: findings,
		Details:  details,
	}
}

func classifyObservedChanges(changes []observedChange, allowed []string) ([]string, []string, []observedChange) {
	approved := make([]string, 0, len(changes))
	unexpected := make([]string, 0, len(changes))
	approvedObserved := make([]observedChange, 0, len(changes))
	for _, change := range changes {
		summary := formatObservedChange(change)
		if pathAllowed(change.Path, allowed) {
			approved = append(approved, summary)
			approvedObserved = append(approvedObserved, change)
			continue
		}
		unexpected = append(unexpected, summary)
	}
	return approved, unexpected, approvedObserved
}

func unexpectedObservedChangeFindings(unexpected []string) []core.Finding {
	if len(unexpected) == 0 {
		return nil
	}
	return []core.Finding{{
		Severity: "critical",
		Message:  fmt.Sprintf("observed file mutations outside approved locations: %s", strings.Join(unexpected, ", ")),
	}}
}

func expectedMutationFindings(missingExpected, undeclaredApproved []string) []core.Finding {
	findings := make([]core.Finding, 0)
	if len(missingExpected) > 0 {
		findings = append(findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("expected file mutations did not occur: %s", strings.Join(missingExpected, ", ")),
		})
	}
	if len(undeclaredApproved) > 0 {
		findings = append(findings, core.Finding{
			Severity: "high",
			Message:  fmt.Sprintf("approved but undeclared file mutations occurred: %s", strings.Join(undeclaredApproved, ", ")),
		})
	}
	return findings
}

func verifyExpectedMutationContent(matchedExpected []expectedMutationCheck) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, check := range matchedExpected {
		if strings.TrimSpace(check.ContentContains) == "" || check.Kind == "deleted" {
			continue
		}
		content, readErr := os.ReadFile(check.Path)
		if readErr != nil {
			findings = append(findings, core.Finding{
				Severity: "high",
				Message:  fmt.Sprintf("read expected mutation target %s: %v", shortenObservedPath(check.Path), readErr),
			})
			continue
		}
		if !strings.Contains(string(content), check.ContentContains) {
			findings = append(findings, core.Finding{
				Severity: "high",
				Message:  fmt.Sprintf("expected %s to contain %q after %s", shortenObservedPath(check.Path), check.ContentContains, check.Kind),
			})
		}
	}
	return findings
}

func shadowStateDetails(resp core.Response, roots []string, changes []observedChange, approved, unexpected []string, expected, matchedExpected []expectedMutationCheck, missingExpected, undeclaredApproved []string) map[string]any {
	details := responseDetails(resp, map[string]any{
		"observed_change_count":   len(changes),
		"approved_change_count":   len(approved),
		"unexpected_change_count": len(unexpected),
		"observed_roots":          renderObservedPaths(roots),
	})
	if len(changes) > 0 {
		details["changed_files"] = renderObservedChanges(changes)
	}
	if len(approved) > 0 {
		details["approved_changes"] = approved
	}
	if len(unexpected) > 0 {
		details["unexpected_changes"] = unexpected
	}
	if len(expected) > 0 {
		details["expected_mutations"] = renderExpectedMutationChecks(expected)
	}
	if len(matchedExpected) > 0 {
		details["matched_expected_mutations"] = renderExpectedMutationChecks(matchedExpected)
	}
	if len(missingExpected) > 0 {
		details["missing_expected_mutations"] = missingExpected
	}
	if len(undeclaredApproved) > 0 {
		details["undeclared_approved_changes"] = undeclaredApproved
	}
	return details
}

type observedFile struct {
	Digest string
}

type observedChange struct {
	Path string
	Kind string
}

type expectedMutationCheck struct {
	Path            string
	Kind            string
	ContentContains string
}

func shadowStateSetupFailure(err error) core.SuiteResult {
	return core.SuiteResult{
		Name:   "shadow-state",
		Passed: false,
		Cases: []core.CaseResult{{
			Name:     "shadow-state-setup",
			Passed:   false,
			Findings: []core.Finding{{Severity: "critical", Message: err.Error()}},
		}},
	}
}

func normalizeObservedPaths(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return nil, err
		}
		out = append(out, filepath.Clean(abs))
	}
	sort.Strings(out)
	return out, nil
}

func normalizeExpectedMutations(mutations []core.ExpectedMutation) ([]expectedMutationCheck, error) {
	out := make([]expectedMutationCheck, 0, len(mutations))
	for _, mutation := range mutations {
		abs, err := filepath.Abs(strings.TrimSpace(mutation.Path))
		if err != nil {
			return nil, err
		}
		out = append(out, expectedMutationCheck{
			Path:            filepath.Clean(abs),
			Kind:            strings.TrimSpace(mutation.Kind),
			ContentContains: mutation.ContentContains,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func captureObservedFiles(roots []string) (map[string]observedFile, error) {
	out := make(map[string]observedFile)
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			if err := captureObservedFile(out, root); err != nil {
				return nil, err
			}
			continue
		}
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			return captureObservedFile(out, path)
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func captureObservedFile(out map[string]observedFile, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	out[filepath.Clean(path)] = observedFile{Digest: hex.EncodeToString(sum[:])}
	return nil
}

func diffObservedFiles(before, after map[string]observedFile) []observedChange {
	paths := make(map[string]struct{}, len(before)+len(after))
	for path := range before {
		paths[path] = struct{}{}
	}
	for path := range after {
		paths[path] = struct{}{}
	}

	changes := make([]observedChange, 0)
	for path := range paths {
		prev, hadPrev := before[path]
		next, hasNext := after[path]
		switch {
		case !hadPrev && hasNext:
			changes = append(changes, observedChange{Path: path, Kind: "created"})
		case hadPrev && !hasNext:
			changes = append(changes, observedChange{Path: path, Kind: "deleted"})
		case hadPrev && hasNext && prev.Digest != next.Digest:
			changes = append(changes, observedChange{Path: path, Kind: "modified"})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func matchExpectedMutations(expected []expectedMutationCheck, observed []observedChange) ([]expectedMutationCheck, []string, []string) {
	if len(expected) == 0 {
		return nil, nil, nil
	}

	expectedByKey := make(map[string]expectedMutationCheck, len(expected))
	for _, item := range expected {
		expectedByKey[mutationKey(item.Path, item.Kind)] = item
	}

	matched := make([]expectedMutationCheck, 0, len(expected))
	undeclared := make([]string, 0)
	for _, change := range observed {
		key := mutationKey(change.Path, change.Kind)
		item, ok := expectedByKey[key]
		if !ok {
			undeclared = append(undeclared, formatObservedChange(change))
			continue
		}
		matched = append(matched, item)
		delete(expectedByKey, key)
	}

	missing := make([]string, 0, len(expectedByKey))
	for _, item := range expectedByKey {
		missing = append(missing, formatExpectedMutationCheck(item))
	}
	sort.Strings(missing)
	sort.Strings(undeclared)
	return matched, missing, undeclared
}

func mutationKey(path, kind string) string {
	return filepath.Clean(path) + "|" + strings.TrimSpace(kind)
}

func pathAllowed(path string, allowed []string) bool {
	for _, prefix := range allowed {
		if hasPathPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func hasPathPrefix(path, prefix string) bool {
	cleanPath := filepath.Clean(path)
	cleanPrefix := filepath.Clean(prefix)
	if cleanPath == cleanPrefix {
		return true
	}
	return strings.HasPrefix(cleanPath, cleanPrefix+string(os.PathSeparator))
}

func renderObservedChanges(changes []observedChange) []string {
	out := make([]string, 0, len(changes))
	for _, change := range changes {
		out = append(out, formatObservedChange(change))
	}
	return out
}

func formatObservedChange(change observedChange) string {
	return fmt.Sprintf("%s:%s", change.Kind, shortenObservedPath(change.Path))
}

func renderExpectedMutationChecks(items []expectedMutationCheck) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, formatExpectedMutationCheck(item))
	}
	return out
}

func formatExpectedMutationCheck(item expectedMutationCheck) string {
	if strings.TrimSpace(item.ContentContains) == "" {
		return fmt.Sprintf("%s:%s", item.Kind, shortenObservedPath(item.Path))
	}
	return fmt.Sprintf("%s:%s contains %q", item.Kind, shortenObservedPath(item.Path), item.ContentContains)
}

func renderObservedPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, shortenObservedPath(path))
	}
	return out
}

func shortenObservedPath(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(wd, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}
