package devtools

import (
	"context"
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
