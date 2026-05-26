package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultCIGovulncheckMode    = "required"
	defaultCIGovulncheckVersion = "v1.3.0"
	defaultCIGocycloVersion     = "v0.6.0"
	defaultCIMinCoverage        = 65.0
	defaultCISemgrepCommand     = "semgrep"
)

var (
	ciCodeChangePattern    = regexp.MustCompile(`^(cleanr/|cmd/|internal/).+\.go$`)
	ciCodeIgnorePattern    = regexp.MustCompile(`(^|/).+_test\.go$|(^|/)doc\.go$`)
	ciTestChangePattern    = regexp.MustCompile(`^(tests/|.*_test\.go$)`)
	ciCICDOnlyPattern      = regexp.MustCompile(`^(\.github/workflows/|\.github/release-please-config\.json$|\.release-please-manifest\.json$)`)
	ciDocSensitivePattern  = regexp.MustCompile(`^(\.goreleaser\.yaml$|\.github/release-please-config\.json$|\.release-please-manifest\.json$|Formula/|README\.md$|docs/|internal/devtools/)`)
	ciDocsChangedPattern   = regexp.MustCompile(`^(README\.md|CONTRIBUTING\.md|docs/)`)
	ciCriticalFilesPattern = regexp.MustCompile(`^(cmd/cleanr/|cmd/cleanr-dev/|cleanr/|internal/cli/|internal/devtools/|internal/mcpserver/|go\.mod|go\.sum|Formula/|\.goreleaser\.yaml|\.github/workflows/.*\.yml|\.github/release-please-config\.json|\.release-please-manifest\.json)`)
	ciDangerousAddPattern  = regexp.MustCompile(`^\+.*(exec\.Command(Context)?\(|http\.(Get|Post)\(|net\.Dial\(|os\.RemoveAll\(|os\.Setenv\(|syscall\.|unsafe \{|panic!\(|TODO|FIXME)`)
	ciGoModAdditionPattern = regexp.MustCompile(`^\+[^+]`)
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
		{name: "gocyclo", fn: func() error { return r.runCIGocyclo(ctx, resolved.GocycloVersion) }},
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
	baseRef := strings.TrimSpace(opts.BaseRef)
	if baseRef == "" {
		baseRef = strings.TrimSpace(os.Getenv("CLEANR_CI_BASE_REF"))
	}
	if baseRef == "" {
		baseRef = strings.TrimSpace(os.Getenv("PR_BASE_REF"))
	}
	if baseRef == "" {
		upstream, err := r.runOutputCommand(ctx, nil, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
		if err == nil {
			baseRef = strings.TrimSpace(upstream)
		}
	}
	if baseRef == "" {
		for _, candidate := range []string{"origin/develop", "origin/main", "origin/master", "develop", "main", "master"} {
			if r.gitRefExists(ctx, candidate) {
				baseRef = candidate
				break
			}
		}
	}
	if baseRef == "" {
		return CIOptions{}, fmt.Errorf("unable to resolve a CI base ref; set CLEANR_CI_BASE_REF or pass -base-ref")
	}
	if !r.gitRefExists(ctx, baseRef) {
		return CIOptions{}, fmt.Errorf("base ref %q does not exist locally", baseRef)
	}

	buildOutput := strings.TrimSpace(opts.BuildOutput)
	if buildOutput == "" {
		buildOutput = filepath.Join("dist", "cleanr-linux-amd64")
	}

	govulncheckMode := strings.TrimSpace(opts.GovulncheckMode)
	if govulncheckMode == "" {
		govulncheckMode = strings.TrimSpace(os.Getenv("GOVULNCHECK_MODE"))
	}
	if govulncheckMode == "" {
		govulncheckMode = defaultCIGovulncheckMode
	}

	govulncheckVersion := strings.TrimSpace(opts.GovulncheckVersion)
	if govulncheckVersion == "" {
		govulncheckVersion = strings.TrimSpace(os.Getenv("GOVULNCHECK_VERSION"))
	}
	if govulncheckVersion == "" {
		govulncheckVersion = defaultCIGovulncheckVersion
	}

	gocycloVersion := strings.TrimSpace(opts.GocycloVersion)
	if gocycloVersion == "" {
		gocycloVersion = strings.TrimSpace(os.Getenv("GOCYCLO_VERSION"))
	}
	if gocycloVersion == "" {
		gocycloVersion = defaultCIGocycloVersion
	}

	minCoverage := opts.MinInternalCoverage
	if minCoverage <= 0 {
		if raw := strings.TrimSpace(os.Getenv("MIN_INTERNAL_COVERAGE")); raw != "" {
			value, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return CIOptions{}, fmt.Errorf("parse MIN_INTERNAL_COVERAGE: %w", err)
			}
			minCoverage = value
		}
	}
	if minCoverage <= 0 {
		minCoverage = defaultCIMinCoverage
	}

	semgrepCommand := strings.TrimSpace(opts.SemgrepCommand)
	if semgrepCommand == "" {
		semgrepCommand = strings.TrimSpace(os.Getenv("SEMGREP"))
	}
	if semgrepCommand == "" {
		semgrepCommand = defaultCISemgrepCommand
	}

	return CIOptions{
		BaseRef:             baseRef,
		BuildOutput:         buildOutput,
		GovulncheckMode:     govulncheckMode,
		GovulncheckVersion:  govulncheckVersion,
		GocycloVersion:      gocycloVersion,
		MinInternalCoverage: minCoverage,
		SemgrepCommand:      semgrepCommand,
	}, nil
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

func (r Runner) runCIGocyclo(ctx context.Context, version string) error {
	if _, err := fmt.Fprintf(r.Stdout, "installing gocyclo %s\n", version); err != nil {
		return err
	}
	if err := r.runCommand(ctx, nil, "go", "install", "github.com/fzipp/gocyclo/cmd/gocyclo@"+version); err != nil {
		return err
	}

	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	targets := make([]string, 0, len(files))
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		if strings.HasPrefix(file, "cleanr/") || strings.HasPrefix(file, "cmd/") || strings.HasPrefix(file, "internal/") {
			targets = append(targets, file)
		}
	}
	if len(targets) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No non-test Go files found.")
		return err
	}

	gopath, err := r.runOutputCommand(ctx, nil, "go", "env", "GOPATH")
	if err != nil {
		return err
	}
	gocycloPath := filepath.Join(strings.TrimSpace(gopath), "bin", "gocyclo")
	out, err := r.runOutputCommand(ctx, nil, gocycloPath, append([]string{"-over", "20"}, targets...)...)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed != "" {
		return fmt.Errorf("gocyclo found functions above the limit:\n%s", trimmed)
	}
	if _, err := fmt.Fprintln(r.Stdout, "gocyclo: ok"); err != nil {
		return err
	}
	return nil
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

func (r Runner) runCISecurity(ctx context.Context, opts CIOptions) error {
	if _, err := fmt.Fprintf(r.Stdout, "installing govulncheck %s\n", opts.GovulncheckVersion); err != nil {
		return err
	}
	if err := r.runCommand(ctx, nil, "go", "install", "golang.org/x/vuln/cmd/govulncheck@"+opts.GovulncheckVersion); err != nil {
		if opts.GovulncheckMode == "required" {
			return fmt.Errorf("install govulncheck: %w", err)
		}
		if _, printErr := fmt.Fprintf(r.Stdout, "warning: govulncheck install failed but mode=%s, continuing\n", opts.GovulncheckMode); printErr != nil {
			return printErr
		}
		return nil
	}

	gopath, err := r.runOutputCommand(ctx, nil, "go", "env", "GOPATH")
	if err != nil {
		return err
	}
	govulncheckPath := filepath.Join(strings.TrimSpace(gopath), "bin", "govulncheck")
	out, govulnErr := r.runOutputCommand(ctx, nil, govulncheckPath, "./...")
	switch {
	case govulnErr == nil:
		if _, err := fmt.Fprintln(r.Stdout, "govulncheck: no known Go vulnerabilities detected"); err != nil {
			return err
		}
	default:
		if _, err := fmt.Fprintln(r.Stdout, "warning: govulncheck reported findings"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(r.Stdout, firstLines(out, 40)); err != nil {
			return err
		}
	}

	changedFiles, err := r.gitChangedFiles(ctx, opts.BaseRef)
	if err != nil {
		return err
	}
	diffText, err := r.gitDiff(ctx, opts.BaseRef)
	if err != nil {
		return err
	}

	if critical := filterMatching(changedFiles, ciCriticalFilesPattern); len(critical) > 0 {
		if _, err := fmt.Fprintln(r.Stdout, "warning: critical cleanr files modified:"); err != nil {
			return err
		}
		for _, file := range critical {
			if _, err := fmt.Fprintln(r.Stdout, file); err != nil {
				return err
			}
		}
	}

	if dangerous := filterMatching(splitNonEmptyLines(diffText), ciDangerousAddPattern); len(dangerous) > 0 {
		if _, err := fmt.Fprintln(r.Stdout, "warning: potentially dangerous additions detected:"); err != nil {
			return err
		}
		for _, line := range dangerous[:min(len(dangerous), 40)] {
			if _, err := fmt.Fprintln(r.Stdout, line); err != nil {
				return err
			}
		}
	}

	goModDiff, err := r.runOutputCommand(ctx, nil, "git", "diff", opts.BaseRef, "--", "go.mod")
	if err != nil {
		return err
	}
	if additions := filterMatching(splitNonEmptyLines(goModDiff), ciGoModAdditionPattern); len(additions) > 0 {
		if _, err := fmt.Fprintln(r.Stdout, "warning: go.mod additions detected:"); err != nil {
			return err
		}
		for _, line := range additions[:min(len(additions), 40)] {
			if _, err := fmt.Fprintln(r.Stdout, line); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(r.Stdout, "security review: manual review still required for warnings above"); err != nil {
		return err
	}
	return nil
}

func (r Runner) runCISemgrep(ctx context.Context, opts CIOptions) error {
	mergeBase, err := r.runOutputCommand(ctx, nil, "git", "merge-base", opts.BaseRef, "HEAD")
	if err != nil {
		return err
	}
	baseline := strings.TrimSpace(mergeBase)
	if baseline == "" {
		return fmt.Errorf("empty merge-base for %s", opts.BaseRef)
	}
	if _, err := fmt.Fprintf(r.Stdout, "running semgrep against baseline %s\n", baseline); err != nil {
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
	mergeBase, err := r.runOutputCommand(ctx, nil, "git", "merge-base", baseRef, "HEAD")
	if err != nil {
		return err
	}
	commitsOut, err := r.runOutputCommand(ctx, nil, "git", "rev-list", strings.TrimSpace(mergeBase)+"..HEAD")
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

func (r Runner) gitChangedFiles(ctx context.Context, baseRef string) ([]string, error) {
	out, err := r.runOutputCommand(ctx, nil, "git", "diff", "--name-only", baseRef, "--")
	if err != nil {
		return nil, err
	}
	files := splitNonEmptyLines(out)

	untrackedOut, err := r.runOutputCommand(ctx, nil, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	for _, file := range splitNonEmptyLines(untrackedOut) {
		if !containsString(files, file) {
			files = append(files, file)
		}
	}
	return files, nil
}

func (r Runner) gitDiff(ctx context.Context, baseRef string) (string, error) {
	diffText, err := r.runOutputCommand(ctx, nil, "git", "diff", baseRef, "--")
	if err != nil {
		return "", err
	}

	untrackedOut, err := r.runOutputCommand(ctx, nil, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(diffText)
	for _, file := range splitNonEmptyLines(untrackedOut) {
		out, err := r.runOutputCommandAllowExitCodes(ctx, nil, map[int]bool{1: true}, "git", "diff", "--no-index", "--", "/dev/null", file)
		if err != nil {
			return "", err
		}
		builder.WriteString(out)
		if out != "" && !strings.HasSuffix(out, "\n") {
			builder.WriteByte('\n')
		}
	}
	return builder.String(), nil
}

func (r Runner) gitRefExists(ctx context.Context, ref string) bool {
	_, err := r.runOutputCommand(ctx, nil, "git", "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func normalizeBaseBranchName(baseRef string) string {
	trimmed := strings.TrimSpace(baseRef)
	trimmed = strings.TrimPrefix(trimmed, "refs/remotes/")
	trimmed = strings.TrimPrefix(trimmed, "refs/heads/")
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func parseCoverageTotal(report string) (float64, error) {
	for _, line := range splitNonEmptyLines(report) {
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}
		value := strings.TrimSuffix(fields[len(fields)-1], "%")
		total, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("parse coverage total %q: %w", value, err)
		}
		return total, nil
	}
	return 0, fmt.Errorf("coverage report missing total line")
}

func splitNonEmptyLines(input string) []string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func filterMatching(lines []string, pattern *regexp.Regexp) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if pattern.MatchString(line) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func firstLines(input string, limit int) string {
	lines := splitNonEmptyLines(input)
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
