package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func (r Runner) runCIGocyclo(ctx context.Context, baseRef, version string) error {
	gocycloPath, err := r.ensureGoTool(ctx, "gocyclo", "github.com/fzipp/gocyclo/cmd/gocyclo", version)
	if err != nil {
		return err
	}

	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}
	targets := filterCIGocycloTargets(changedFiles)
	if len(targets) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No changed non-test Go files for gocyclo.")
		return err
	}

	out, err := r.runOutputCommand(ctx, nil, gocycloPath, append([]string{"-over", "20"}, targets...)...)
	if err != nil {
		return err
	}

	currentFindings, err := parseGocycloFindings(out)
	if err != nil {
		return err
	}
	if len(currentFindings) == 0 {
		if _, err := fmt.Fprintln(r.Stdout, "gocyclo: ok"); err != nil {
			return err
		}
		return nil
	}

	baseFindings, err := r.loadBaseGocycloFindings(ctx, baseRef, gocycloPath, targets)
	if err != nil {
		return err
	}
	regressions := diffGocycloRegressions(currentFindings, baseFindings)
	if len(regressions) > 0 {
		return fmt.Errorf("gocyclo found new or worsened functions above the limit:\n%s", strings.Join(regressions, "\n"))
	}
	if _, err := fmt.Fprintf(r.Stdout, "gocyclo: no new complexity regressions (%d existing baseline findings tolerated)\n", len(baseFindings)); err != nil {
		return err
	}
	return nil
}

func (r Runner) loadBaseGocycloFindings(ctx context.Context, baseRef, gocycloPath string, targets []string) (map[string]gocycloFinding, error) {
	baseDir, err := os.MkdirTemp("", "cleanr-ci-gocyclo-base-*")
	if err != nil {
		return nil, fmt.Errorf("create gocyclo base temp dir: %w", err)
	}
	defer os.RemoveAll(baseDir)

	baseTargets := make([]string, 0, len(targets))
	for _, target := range targets {
		existsOut, err := r.runOutputCommand(ctx, nil, "git", "ls-tree", "-r", "--name-only", baseRef, "--", target)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(existsOut) == "" {
			continue
		}

		content, err := r.runOutputCommand(ctx, nil, "git", "show", baseRef+":"+target)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(baseDir, filepath.FromSlash(target))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create base dir for %s: %w", target, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write base file %s: %w", target, err)
		}
		baseTargets = append(baseTargets, target)
	}

	if len(baseTargets) == 0 {
		return map[string]gocycloFinding{}, nil
	}

	baseRunner := NewRunner(baseDir, r.Stdout, r.Stderr)
	out, err := baseRunner.runOutputCommand(ctx, nil, gocycloPath, append([]string{"-over", "20"}, baseTargets...)...)
	if err != nil {
		return nil, err
	}
	return parseGocycloFindings(out)
}

func filterCIGocycloTargets(files []string) []string {
	targets := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		if strings.HasPrefix(file, "cleanr/") || strings.HasPrefix(file, "cmd/") || strings.HasPrefix(file, "internal/") {
			targets = append(targets, file)
		}
	}
	return targets
}

func parseGocycloFindings(raw string) (map[string]gocycloFinding, error) {
	findings := make(map[string]gocycloFinding)
	for _, line := range splitNonEmptyLines(raw) {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil, fmt.Errorf("parse gocyclo output line %q", line)
		}

		complexity, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("parse gocyclo complexity from %q: %w", line, err)
		}
		location := fields[len(fields)-1]
		path := location
		if idx := strings.Index(location, ":"); idx >= 0 {
			path = location[:idx]
		}

		finding := gocycloFinding{
			Complexity: complexity,
			Path:       path,
			Package:    fields[1],
			Symbol:     strings.Join(fields[2:len(fields)-1], " "),
			Raw:        line,
		}
		findings[gocycloFindingKey(finding)] = finding
	}
	return findings, nil
}

func diffGocycloRegressions(current, base map[string]gocycloFinding) []string {
	regressions := make([]string, 0, len(current))
	for key, finding := range current {
		baseline, ok := base[key]
		if ok && finding.Complexity <= baseline.Complexity {
			continue
		}
		regressions = append(regressions, finding.Raw)
	}
	sort.Strings(regressions)
	return regressions
}

func gocycloFindingKey(finding gocycloFinding) string {
	return finding.Path + "|" + finding.Package + "|" + finding.Symbol
}
