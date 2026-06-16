package cli

import (
	"fmt"
	"io"
	"os"

	setuppkg "github.com/devr-tools/cleanr/internal/cli/setup"
	versionpkg "github.com/devr-tools/cleanr/internal/version"
)

var version = versionpkg.Number

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "run":
		return runCmd(args[1:], stdout, stderr)
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

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: cleanr <run|explain|generate|trends|dataset|sync|plugins|github|snapshot|validate|init|setup|mcp|version> [flags]")
}
