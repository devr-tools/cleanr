package devtools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (r Runner) runCommand(ctx context.Context, env map[string]string, name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("find %s: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = r.WorkDir
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func (r Runner) runOutputCommand(ctx context.Context, env map[string]string, name string, args ...string) (string, error) {
	return r.runOutputCommandAllowExitCodes(ctx, env, nil, name, args...)
}

func (r Runner) runOutputCommandAllowExitCodes(ctx context.Context, env map[string]string, allowedExitCodes map[int]bool, name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("find %s: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = r.WorkDir
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && allowedExitCodes[exitErr.ExitCode()] {
			return string(out), nil
		}
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return string(out), fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, trimmed)
	}
	return string(out), nil
}
