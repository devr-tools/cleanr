package devtools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func (r Runner) Format(ctx context.Context) error {
	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	if err := validateGoFileLayout(files); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no Go files found")
	}
	if _, err := fmt.Fprintln(r.Stdout, "formatting Go files"); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, "gofmt", append([]string{"-w"}, files...)...)
}

func (r Runner) FormatCheck(ctx context.Context) error {
	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	if err := validateGoFileLayout(files); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no Go files found")
	}
	path, err := exec.LookPath("gofmt")
	if err != nil {
		return fmt.Errorf("find gofmt: %w", err)
	}
	cmd := exec.CommandContext(ctx, path, append([]string{"-l"}, files...)...)
	cmd.Dir = r.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run gofmt -l: %w", err)
	}
	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		return fmt.Errorf("unformatted Go files:\n%s", trimmed)
	}
	if _, err := fmt.Fprintln(r.Stdout, "format check: ok"); err != nil {
		return err
	}
	return nil
}
