package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type CLI struct {
	cfg core.TargetConfig
}

func NewCLI(cfg core.TargetConfig) *CLI {
	return &CLI{cfg: cfg}
}

func (t *CLI) Invoke(ctx context.Context, req core.Request) core.Response {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.cfg.Timeout()
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(req)
	if err != nil {
		return core.Response{Err: err}
	}

	start := time.Now()
	cmd := exec.CommandContext(cmdCtx, t.cfg.CLI.Command, t.cfg.CLI.Args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = append([]string(nil), os.Environ()...)
	for key, value := range t.cfg.CLI.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	latency := time.Since(start)
	resp := core.Response{
		Body:         stdout.Bytes(),
		Text:         stdout.String(),
		Stderr:       stderr.String(),
		Latency:      latency,
		Normalized:   core.ProviderResponse{Provider: "cli"},
		ExtractError: nil,
	}
	if t.cfg.ResponseField != "" {
		resp.Text, resp.ExtractError = extractResponseField(resp.Body, t.cfg.ResponseField)
	}
	if runErr == nil {
		return resp
	}

	resp.ExitCode = cliExitCode(runErr)
	resp.Err = fmt.Errorf("cli command failed with exit code %d", resp.ExitCode)
	return resp
}

func cliExitCode(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 1
	}
	return exitErr.ExitCode()
}
