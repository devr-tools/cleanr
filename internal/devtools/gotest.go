package devtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type goTestEvent struct {
	Action string `json:"Action"`
	Output string `json:"Output"`
}

func (r Runner) runGoTestFiltered(ctx context.Context, args ...string) error {
	path, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("find go: %w", err)
	}

	cmdArgs := append([]string{"test", "-json"}, args...)
	cmd := exec.CommandContext(ctx, path, cmdArgs...)
	cmd.Dir = r.WorkDir
	cmd.Stderr = r.Stderr
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("go test stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(cmdArgs, " "), err)
	}

	dec := json.NewDecoder(stdout)
	for {
		var event goTestEvent
		if err := dec.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = cmd.Wait()
			return fmt.Errorf("decode go test json: %w", err)
		}
		if event.Action != "output" {
			continue
		}
		if strings.Contains(event.Output, "[no test files]") {
			continue
		}
		if _, err := io.WriteString(r.Stdout, event.Output); err != nil {
			_ = cmd.Wait()
			return err
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(cmdArgs, " "), err)
	}
	return nil
}
