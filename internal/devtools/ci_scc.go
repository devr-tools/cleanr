package devtools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type sccLanguageReport struct {
	Files []sccFileReport `json:"Files"`
}

type sccFileReport struct {
	Code     int    `json:"Code"`
	Lines    int    `json:"Lines"`
	Location string `json:"Location"`
}

func (r Runner) runCISCC(ctx context.Context, baseRef, version string, maxCodeLines int) error {
	sccPath, err := r.ensureGoTool(ctx, "scc", "github.com/boyter/scc/v3", version)
	if err != nil {
		return err
	}

	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}
	targets := filterCIGocycloTargets(changedFiles)
	if len(targets) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No changed non-test Go files for scc.")
		return err
	}

	currentStats, err := r.runSCCReport(ctx, sccPath, r.WorkDir, targets)
	if err != nil {
		return err
	}
	baseStats, err := r.loadBaseSCCStats(ctx, baseRef, sccPath, targets)
	if err != nil {
		return err
	}

	regressions := diffSCCRegressions(currentStats, baseStats, maxCodeLines)
	if len(regressions) > 0 {
		return fmt.Errorf(
			"scc found new or worsened files above the code-line limit (%d):\n%s",
			maxCodeLines,
			strings.Join(regressions, "\n"),
		)
	}

	if _, err := fmt.Fprintf(
		r.Stdout,
		"scc: no new file-size regressions above %d code lines (%d changed files checked)\n",
		maxCodeLines,
		len(currentStats),
	); err != nil {
		return err
	}
	return nil
}

func (r Runner) runSCCReport(ctx context.Context, sccPath, workDir string, targets []string) (map[string]sccFileReport, error) {
	baseRunner := NewRunner(workDir, r.Stdout, r.Stderr)
	out, err := baseRunner.runOutputCommand(ctx, nil, sccPath, append([]string{"--by-file", "--format", "json"}, targets...)...)
	if err != nil {
		return nil, err
	}
	return parseSCCReport(out)
}

func (r Runner) loadBaseSCCStats(ctx context.Context, baseRef, sccPath string, targets []string) (map[string]sccFileReport, error) {
	baseDir, err := os.MkdirTemp("", "cleanr-ci-scc-base-*")
	if err != nil {
		return nil, fmt.Errorf("create scc base temp dir: %w", err)
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
		return map[string]sccFileReport{}, nil
	}

	return r.runSCCReport(ctx, sccPath, baseDir, baseTargets)
}

func parseSCCReport(raw string) (map[string]sccFileReport, error) {
	var reports []sccLanguageReport
	if err := json.Unmarshal([]byte(raw), &reports); err != nil {
		return nil, fmt.Errorf("parse scc json: %w", err)
	}

	files := make(map[string]sccFileReport)
	for _, report := range reports {
		for _, file := range report.Files {
			path := filepath.ToSlash(strings.TrimSpace(file.Location))
			if path == "" {
				continue
			}
			file.Location = path
			files[path] = file
		}
	}
	return files, nil
}

func diffSCCRegressions(current, base map[string]sccFileReport, maxCodeLines int) []string {
	regressions := make([]string, 0, len(current))
	for path, finding := range current {
		if finding.Code <= maxCodeLines {
			continue
		}

		baseline, ok := base[path]
		if ok && baseline.Code > maxCodeLines && finding.Code <= baseline.Code {
			continue
		}

		if ok {
			regressions = append(
				regressions,
				fmt.Sprintf("%s code=%d lines=%d baseline_code=%d baseline_lines=%d", path, finding.Code, finding.Lines, baseline.Code, baseline.Lines),
			)
			continue
		}

		regressions = append(regressions, fmt.Sprintf("%s code=%d lines=%d", path, finding.Code, finding.Lines))
	}
	sort.Strings(regressions)
	return regressions
}
