package devtools

import (
	"context"
	"fmt"
	"os"
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
	return r.runCommand(
		ctx,
		nil,
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
