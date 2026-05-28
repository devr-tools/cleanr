package devtools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (r Runner) runCISecurity(ctx context.Context, opts CIOptions) error {
	if err := r.runGovulncheck(ctx, opts); err != nil {
		return err
	}

	changedFiles, err := r.gitChangedFiles(ctx, opts.BaseRef)
	if err != nil {
		return err
	}
	diffText, err := r.gitDiff(ctx, opts.BaseRef)
	if err != nil {
		return err
	}
	if err := r.printSecurityFileWarnings(ctx, changedFiles, diffText, opts.BaseRef); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(r.Stdout, "security review: manual review still required for warnings above"); err != nil {
		return err
	}
	return nil
}

func (r Runner) runCISemgrep(ctx context.Context, opts CIOptions) error {
	if _, err := exec.LookPath(opts.SemgrepCommand); err != nil {
		if _, printErr := fmt.Fprintf(r.Stdout, "warning: semgrep is unavailable, skipping local semgrep scan (%v)\n", err); printErr != nil {
			return printErr
		}
		return nil
	}

	baseline, hasMergeBase, err := r.gitDiffBase(ctx, opts.BaseRef)
	if err != nil {
		return err
	}
	label := "baseline"
	if !hasMergeBase {
		label = "fallback baseline"
	}
	if _, err := fmt.Fprintf(r.Stdout, "running semgrep against %s %s\n", label, baseline); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, opts.SemgrepCommand, "scan", "--config", "auto", "--baseline-commit", baseline, "--error")
}

func (r Runner) runCIDocReview(ctx context.Context, baseRef string) error {
	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}

	ciCDOnly := filterMatching(changedFiles, ciCICDOnlyPattern)
	docSensitive := filterMatching(changedFiles, ciDocSensitivePattern)
	docsChanged := filterMatching(changedFiles, ciDocsChangedPattern)

	switch {
	case len(docSensitive) == 0 && len(ciCDOnly) > 0:
		_, err := fmt.Fprintln(r.Stdout, "Only CI/CD automation files changed. Documentation updates are not required.")
		return err
	case len(docSensitive) == 0:
		_, err := fmt.Fprintln(r.Stdout, "No install or release changes that require documentation review.")
		return err
	case len(docsChanged) > 0:
		if _, err := fmt.Fprintln(r.Stdout, "Relevant documentation files were updated:"); err != nil {
			return err
		}
		for _, file := range docsChanged {
			if _, err := fmt.Fprintln(r.Stdout, file); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("install or release changes require updates in README.md, CONTRIBUTING.md, or docs/")
	}
}

func (r Runner) runCIDCO(ctx context.Context, baseRef string) error {
	baseline, _, err := r.gitDiffBase(ctx, baseRef)
	if err != nil {
		return err
	}
	commitsOut, err := r.runOutputCommand(ctx, nil, "git", "rev-list", baseline+"..HEAD")
	if err != nil {
		return err
	}
	commits := splitNonEmptyLines(commitsOut)
	if len(commits) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No local commits ahead of the base ref; DCO check skipped.")
		return err
	}

	var missing []string
	for _, commit := range commits {
		body, err := r.runOutputCommand(ctx, nil, "git", "show", "-s", "--format=%B", commit)
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(body), "signed-off-by:") {
			missing = append(missing, commit)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("commits missing Signed-off-by trailers: %s", strings.Join(missing, ", "))
	}
	if _, err := fmt.Fprintln(r.Stdout, "dco: ok"); err != nil {
		return err
	}
	return nil
}

func (r Runner) ensureGoTool(ctx context.Context, binaryName, modulePath, version string) (string, error) {
	gopath, err := r.runOutputCommand(ctx, nil, "go", "env", "GOPATH")
	if err != nil {
		return "", err
	}
	toolPath := filepath.Join(strings.TrimSpace(gopath), "bin", binaryName)
	if info, err := os.Stat(toolPath); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
		if _, err := fmt.Fprintf(r.Stdout, "using existing %s at %s\n", binaryName, toolPath); err != nil {
			return "", err
		}
		return toolPath, nil
	}

	if _, err := fmt.Fprintf(r.Stdout, "installing %s %s\n", binaryName, version); err != nil {
		return "", err
	}
	if err := r.runCommand(ctx, nil, "go", "install", modulePath+"@"+version); err != nil {
		return "", err
	}
	return toolPath, nil
}

func (r Runner) runGovulncheck(ctx context.Context, opts CIOptions) error {
	govulncheckPath, err := r.ensureGoTool(ctx, "govulncheck", "golang.org/x/vuln/cmd/govulncheck", opts.GovulncheckVersion)
	if err != nil {
		if _, printErr := fmt.Fprintf(r.Stdout, "warning: govulncheck unavailable, skipping local scan (%v)\n", err); printErr != nil {
			return printErr
		}
		return nil
	}

	out, govulnErr := r.runOutputCommand(ctx, nil, govulncheckPath, "./...")
	if govulnErr == nil {
		_, err := fmt.Fprintln(r.Stdout, "govulncheck: no known Go vulnerabilities detected")
		return err
	}

	if _, err := fmt.Fprintln(r.Stdout, "warning: govulncheck reported findings"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(r.Stdout, firstLines(out, 40)); err != nil {
		return err
	}
	return nil
}

func (r Runner) printSecurityFileWarnings(ctx context.Context, changedFiles []string, diffText, baseRef string) error {
	if err := r.printWarningLines("warning: critical cleanr files modified:", filterMatching(changedFiles, ciCriticalFilesPattern)); err != nil {
		return err
	}
	if err := r.printWarningLines("warning: potentially dangerous additions detected:", filterMatching(splitNonEmptyLines(diffText), ciDangerousAddPattern)); err != nil {
		return err
	}

	goModDiff, err := r.runOutputCommand(ctx, nil, "git", "diff", baseRef, "--", "go.mod")
	if err != nil {
		return err
	}
	return r.printWarningLines("warning: go.mod additions detected:", filterMatching(splitNonEmptyLines(goModDiff), ciGoModAdditionPattern))
}

func (r Runner) printWarningLines(header string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(r.Stdout, header); err != nil {
		return err
	}
	for _, line := range lines[:min(len(lines), 40)] {
		if _, err := fmt.Fprintln(r.Stdout, line); err != nil {
			return err
		}
	}
	return nil
}
