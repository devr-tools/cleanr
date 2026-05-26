package devtools

import (
	"context"
	"fmt"
	"strings"
)

func (r Runner) runCIGolangCILint(ctx context.Context, baseRef, version string) error {
	golangciLintPath, err := r.ensureGoTool(ctx, "golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", version)
	if err != nil {
		return err
	}

	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}
	targets := filterCIGocycloTargets(changedFiles)
	if len(targets) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No changed non-test Go files for golangci-lint.")
		return err
	}

	mergeBase, err := r.runOutputCommand(ctx, nil, "git", "merge-base", baseRef, "HEAD")
	if err != nil {
		return err
	}
	baseline := strings.TrimSpace(mergeBase)
	if baseline == "" {
		return fmt.Errorf("empty merge-base for %s", baseRef)
	}

	if _, err := fmt.Fprintf(r.Stdout, "running golangci-lint against baseline %s\n", baseline); err != nil {
		return err
	}

	return r.runCommand(
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
}
