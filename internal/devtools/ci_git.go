package devtools

import (
	"context"
	"os"
	"strings"
)

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

func (r Runner) resolveCIBaseRef(ctx context.Context, explicit string) (string, error) {
	for _, candidate := range []string{
		strings.TrimSpace(explicit),
		strings.TrimSpace(os.Getenv("CLEANR_CI_BASE_REF")),
		strings.TrimSpace(os.Getenv("PR_BASE_REF")),
	} {
		if candidate != "" {
			return candidate, nil
		}
	}

	upstream, err := r.runOutputCommand(ctx, nil, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && strings.TrimSpace(upstream) != "" {
		return strings.TrimSpace(upstream), nil
	}

	for _, candidate := range []string{"origin/develop", "origin/main", "origin/master", "develop", "main", "master"} {
		if r.gitRefExists(ctx, candidate) {
			return candidate, nil
		}
	}
	return "", nil
}
