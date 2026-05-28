package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var syncExecCommandContext = exec.CommandContext
var syncLookPath = exec.LookPath

type gitHubPROptions struct {
	Files         []string
	Branch        string
	Base          string
	Title         string
	Body          string
	CommitMessage string
}

func createGitHubPR(ctx context.Context, opts gitHubPROptions) error {
	if len(opts.Files) == 0 {
		return fmt.Errorf("create github pr: no files to include")
	}
	if _, err := syncLookPath("git"); err != nil {
		return fmt.Errorf("create github pr: git is not available")
	}
	if _, err := syncLookPath("gh"); err != nil {
		return fmt.Errorf("create github pr: gh is not available")
	}
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		branch = "cleanr-sync-" + time.Now().UTC().Format("20060102-150405")
	}
	if err := runSyncCommand(ctx, "git", "checkout", "-b", branch); err != nil {
		return err
	}
	addArgs := append([]string{"add"}, opts.Files...)
	if err := runSyncCommand(ctx, "git", addArgs...); err != nil {
		return err
	}
	if err := runSyncCommand(ctx, "git", "commit", "-m", strings.TrimSpace(opts.CommitMessage)); err != nil {
		return err
	}
	args := []string{"pr", "create", "--title", strings.TrimSpace(opts.Title), "--body", strings.TrimSpace(opts.Body)}
	if strings.TrimSpace(opts.Base) != "" {
		args = append(args, "--base", strings.TrimSpace(opts.Base))
	}
	if err := runSyncCommand(ctx, "gh", args...); err != nil {
		return err
	}
	return nil
}

func runSyncCommand(ctx context.Context, name string, args ...string) error {
	cmd := syncExecCommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
	}
	return nil
}
