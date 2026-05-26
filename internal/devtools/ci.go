package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultCIGovulncheckMode    = "required"
	defaultCIGovulncheckVersion = "v1.3.0"
	defaultCIGocycloVersion     = "v0.6.0"
	defaultCISCCVersion         = "v3.7.0"
	defaultCIMaxFileCodeLines   = 400
	defaultCIGolangciVersion    = "v2.12.2"
	defaultCIMinCoverage        = 65.0
	defaultCISemgrepCommand     = "semgrep"
)

func (r Runner) CI(ctx context.Context, opts CIOptions) error {
	resolved, err := r.resolveCIOptions(ctx, opts)
	if err != nil {
		return err
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{name: "test-presence", fn: func() error { return r.checkTestPresence(ctx, resolved.BaseRef) }},
		{name: "fmt", fn: func() error { return r.FormatCheck(ctx) }},
		{name: "vet", fn: func() error { return r.Lint(ctx) }},
		{name: "gocyclo", fn: func() error { return r.runCIGocyclo(ctx, resolved.BaseRef, resolved.GocycloVersion) }},
		{name: "scc", fn: func() error { return r.runCISCC(ctx, resolved.BaseRef, resolved.SCCVersion, resolved.MaxFileCodeLines) }},
		{name: "golangci-lint", fn: func() error { return r.runCIGolangCILint(ctx, resolved.BaseRef, resolved.GolangCILintVersion) }},
		{name: "test", fn: func() error { return r.Test(ctx) }},
		{name: "build", fn: func() error { return r.runCIBuild(ctx, resolved.BuildOutput) }},
		{name: "coverage", fn: func() error { return r.runCICoverage(ctx, resolved.MinInternalCoverage) }},
		{name: "security", fn: func() error { return r.runCISecurity(ctx, resolved) }},
		{name: "semgrep", fn: func() error { return r.runCISemgrep(ctx, resolved) }},
	}

	if normalizeBaseBranchName(resolved.BaseRef) == "develop" {
		steps = append(steps,
			struct {
				name string
				fn   func() error
			}{name: "doc-review", fn: func() error { return r.runCIDocReview(ctx, resolved.BaseRef) }},
			struct {
				name string
				fn   func() error
			}{name: "dco", fn: func() error { return r.runCIDCO(ctx, resolved.BaseRef) }},
		)
	}

	for _, step := range steps {
		if _, err := fmt.Fprintf(r.Stdout, "==> %s\n", step.name); err != nil {
			return err
		}
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	if _, err := fmt.Fprintln(r.Stdout, "local ci: ok"); err != nil {
		return err
	}
	return nil
}

func (r Runner) resolveCIOptions(ctx context.Context, opts CIOptions) (CIOptions, error) {
	baseRef, err := r.resolveCIBaseRef(ctx, opts.BaseRef)
	if err != nil {
		return CIOptions{}, err
	}
	if baseRef == "" {
		return CIOptions{}, fmt.Errorf("unable to resolve a CI base ref; set CLEANR_CI_BASE_REF or pass -base-ref")
	}
	if !r.gitRefExists(ctx, baseRef) {
		return CIOptions{}, fmt.Errorf("base ref %q does not exist locally", baseRef)
	}

	return CIOptions{
		BaseRef:             baseRef,
		BuildOutput:         resolveCIString(opts.BuildOutput, "", filepath.Join("dist", "cleanr-linux-amd64")),
		GovulncheckMode:     resolveCIString(opts.GovulncheckMode, "GOVULNCHECK_MODE", defaultCIGovulncheckMode),
		GovulncheckVersion:  resolveCIString(opts.GovulncheckVersion, "GOVULNCHECK_VERSION", defaultCIGovulncheckVersion),
		GocycloVersion:      resolveCIString(opts.GocycloVersion, "GOCYCLO_VERSION", defaultCIGocycloVersion),
		SCCVersion:          resolveCIString(opts.SCCVersion, "SCC_VERSION", defaultCISCCVersion),
		MaxFileCodeLines:    resolveCIMaxFileCodeLines(opts.MaxFileCodeLines),
		GolangCILintVersion: resolveCIString(opts.GolangCILintVersion, "GOLANGCI_LINT_VERSION", defaultCIGolangciVersion),
		MinInternalCoverage: resolveCICoverageThreshold(opts.MinInternalCoverage),
		SemgrepCommand:      resolveCIString(opts.SemgrepCommand, "SEMGREP", defaultCISemgrepCommand),
	}, nil
}

func (r Runner) CISCC(ctx context.Context, opts CIOptions) error {
	resolved, err := r.resolveCIOptions(ctx, opts)
	if err != nil {
		return err
	}
	return r.runCISCC(ctx, resolved.BaseRef, resolved.SCCVersion, resolved.MaxFileCodeLines)
}

func (r Runner) CIGolangCILint(ctx context.Context, opts CIOptions) error {
	resolved, err := r.resolveCIOptions(ctx, opts)
	if err != nil {
		return err
	}
	return r.runCIGolangCILint(ctx, resolved.BaseRef, resolved.GolangCILintVersion)
}

func (r Runner) checkTestPresence(ctx context.Context, baseRef string) error {
	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}

	var codeChanges []string
	var testChanges []string
	for _, file := range changedFiles {
		switch {
		case ciCodeChangePattern.MatchString(file) && !ciCodeIgnorePattern.MatchString(file):
			codeChanges = append(codeChanges, file)
		case ciTestChangePattern.MatchString(file):
			testChanges = append(testChanges, file)
		}
	}

	if len(codeChanges) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No Go source changes that require a test presence check.")
		return err
	}

	if _, err := fmt.Fprintln(r.Stdout, "Go source files changed:"); err != nil {
		return err
	}
	for _, file := range codeChanges {
		if _, err := fmt.Fprintln(r.Stdout, file); err != nil {
			return err
		}
	}
	if len(testChanges) > 0 {
		if _, err := fmt.Fprintln(r.Stdout, "\nMatching test updates detected:"); err != nil {
			return err
		}
		for _, file := range testChanges {
			if _, err := fmt.Fprintln(r.Stdout, file); err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("Go source changed without any updates under tests/ or *_test.go files")
}

func (r Runner) runCIBuild(ctx context.Context, output string) error {
	outputPath := resolvePath(r.WorkDir, output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create build output dir: %w", err)
	}
	if _, err := fmt.Fprintf(r.Stdout, "building linux/amd64 snapshot %s\n", outputPath); err != nil {
		return err
	}
	env := map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
		"GOARCH":      "amd64",
	}
	return r.runCommand(ctx, env, "go", "build", "-trimpath", "-o", outputPath, "./cmd/cleanr")
}

func (r Runner) runCICoverage(ctx context.Context, minCoverage float64) error {
	coveragePath := resolvePath(r.WorkDir, filepath.Join("dist", ".coverage.internal.out"))
	if err := os.MkdirAll(filepath.Dir(coveragePath), 0o755); err != nil {
		return fmt.Errorf("create coverage output dir: %w", err)
	}
	if _, err := fmt.Fprintf(r.Stdout, "checking internal coverage >= %.1f%%\n", minCoverage); err != nil {
		return err
	}
	if err := r.runCommand(ctx, nil, "go", "test", "./...", "-coverpkg=./internal/...", "-coverprofile="+coveragePath); err != nil {
		return err
	}
	out, err := r.runOutputCommand(ctx, nil, "go", "tool", "cover", "-func="+coveragePath)
	if err != nil {
		return err
	}
	total, err := parseCoverageTotal(out)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Stdout, "total internal coverage: %.1f%%\n", total); err != nil {
		return err
	}
	if total < minCoverage {
		return fmt.Errorf("total internal coverage %.1f%% is below %.1f%%", total, minCoverage)
	}
	return nil
}
