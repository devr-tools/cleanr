package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	setuppkg "github.com/devr-tools/cleanr/internal/cli/setup"
	versionpkg "github.com/devr-tools/cleanr/internal/version"
)

var version = versionpkg.Number

func Run(args []string, stdout, stderr io.Writer) int {
	args, debug := extractDebugFlag(args)
	configureLogger(stderr, debug)

	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	slog.Debug("cli command starting", "command", args[0], "version", version)

	switch args[0] {
	case "run":
		return runCmd(args[1:], stdout, stderr)
	case "watch":
		return watchCmd(args[1:], stdout, stderr)
	case "explain":
		return explainCmd(args[1:], stdout, stderr)
	case "generate":
		return generateCmd(args[1:], stdout, stderr)
	case "trends":
		return trendsCmd(args[1:], stdout, stderr)
	case "dataset":
		return datasetCmd(args[1:], stdout, stderr)
	case "sync":
		return syncCmd(args[1:], stdout, stderr)
	case "plugins":
		return pluginsCmd(args[1:], stdout, stderr)
	case "github":
		return githubCmd(args[1:], stdout, stderr)
	case "snapshot":
		return snapshotCmd(args[1:], stdout, stderr)
	case "validate":
		return validateCmd(args[1:], stdout, stderr)
	case "init":
		return initCmd(args[1:], stdout, stderr)
	case "setup":
		return setuppkg.Run(args[1:], os.Stdin, stdout, stderr)
	case "mcp":
		return mcpCmd(args[1:], stdout, stderr)
	case "version":
		_, _ = fmt.Fprintf(stdout, "cleanr %s\n", version)
		return 0
	default:
		usage(stderr)
		return 2
	}
}

// extractDebugFlag removes the global -v/--debug verbosity flag from args
// wherever it appears (before or after the subcommand) so per-command flag
// sets never see it, and reports whether it was present.
func extractDebugFlag(args []string) ([]string, bool) {
	debug := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-v", "--v", "-debug", "--debug", "-verbose", "--verbose":
			debug = true
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, debug
}

// configureLogger installs the process-wide slog logger scoped to the CLI
// layer. It defaults to warnings and errors on stderr; -v/--debug lowers the
// level to Debug for command-lifecycle diagnostics.
func configureLogger(stderr io.Writer, debug bool) {
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: cleanr [-v|--debug] <run|watch|explain|generate|trends|dataset|sync|plugins|github|snapshot|validate|init|setup|mcp|version> [flags]")
}
