package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultCodeGuardConfigPath = ".codeguard"

func (r Runner) PackageCodeGuard(ctx context.Context, opts CIOptions) error {
	resolved, err := r.resolveCIOptions(ctx, opts)
	if err != nil {
		return err
	}
	return r.runPackageCodeGuard(ctx, resolved)
}

func (r Runner) runPackageCodeGuard(ctx context.Context, opts CIOptions) error {
	if _, err := os.Stat(resolvePath(r.WorkDir, defaultCodeGuardConfigPath)); err != nil {
		return fmt.Errorf("codeguard config %q not found: %w", defaultCodeGuardConfigPath, err)
	}

	codeguardPath, err := r.ensureGoTool(ctx, "codeguard", "github.com/devr-tools/codeguard/cmd/codeguard", opts.CodeGuardVersion)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(r.Stdout, "running codeguard package against %s\n", opts.BaseRef); err != nil {
		return err
	}
	env := map[string]string{}
	if gopath, err := r.runOutputCommand(ctx, nil, "go", "env", "GOPATH"); err == nil {
		goBin := filepath.Join(strings.TrimSpace(gopath), "bin")
		pathValue := strings.TrimSpace(os.Getenv("PATH"))
		if pathValue == "" {
			env["PATH"] = goBin
		} else {
			env["PATH"] = goBin + string(os.PathListSeparator) + pathValue
		}
	}
	return r.runCommand(
		ctx,
		env,
		codeguardPath,
		"scan",
		"-config",
		defaultCodeGuardConfigPath,
		"-mode",
		"diff",
		"-base-ref",
		opts.BaseRef,
		"-format",
		"text",
	)
}
