package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func githubCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "github error: expected one of doctor or auth")
		return 2
	}
	switch args[0] {
	case "doctor":
		return githubDoctorCmd(args[1:], stdout, stderr)
	case "auth":
		return githubAuthCmd(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "github error: unsupported subcommand %s\n", args[0])
		return 2
	}
}

func githubDoctorCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("github doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	_, _ = fmt.Fprintln(stdout, "GitHub integration doctor")
	if _, err := syncLookPath("gh"); err != nil {
		_, _ = fmt.Fprintln(stdout, "- GitHub CLI: missing")
		_, _ = fmt.Fprintln(stdout, "- Auth: unavailable")
		_, _ = fmt.Fprintln(stdout, "- Next step: install GitHub CLI and run `cleanr github auth`")
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "- GitHub CLI: available")
	statusOutput, err := runSyncCommandOutput(context.Background(), "gh", "auth", "status")
	if err != nil {
		_, _ = fmt.Fprintln(stdout, "- Auth: not ready")
		if strings.TrimSpace(err.Error()) != "" {
			_, _ = fmt.Fprintf(stdout, "- Detail: %s\n", err)
		}
		_, _ = fmt.Fprintln(stdout, "- Next step: run `cleanr github auth` locally, or provide a token-backed gh session in CI")
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "- Auth: ok")
	if repo := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); repo != "" {
		_, _ = fmt.Fprintf(stdout, "- Repository: `%s`\n", repo)
	}
	if eventPath := strings.TrimSpace(os.Getenv("GITHUB_EVENT_PATH")); eventPath != "" {
		if number, resolveErr := resolveGitHubPRNumber(0); resolveErr == nil && number > 0 {
			_, _ = fmt.Fprintf(stdout, "- Pull request context: `#%d`\n", number)
		} else {
			_, _ = fmt.Fprintf(stdout, "- Pull request context: event file present at `%s`\n", eventPath)
		}
	}
	if strings.TrimSpace(statusOutput) != "" {
		_, _ = fmt.Fprintln(stdout, "- gh auth status:")
		for _, line := range strings.Split(strings.TrimSpace(statusOutput), "\n") {
			_, _ = fmt.Fprintf(stdout, "  %s\n", line)
		}
	}
	return 0
}

func githubAuthCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("github auth", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if _, err := syncLookPath("gh"); err != nil {
		_, _ = fmt.Fprintln(stderr, "github auth error: gh is not available")
		return 2
	}
	_, _ = fmt.Fprintln(stdout, "launching GitHub CLI authentication")
	if err := runSyncCommand(context.Background(), "gh", "auth", "login"); err != nil {
		_, _ = fmt.Fprintf(stderr, "github auth error: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintln(stdout, "GitHub CLI authentication completed")
	return 0
}
