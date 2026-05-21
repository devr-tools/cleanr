package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"cleanr/cleanr/core"
)

type StateAdapterDelta struct {
	ToolCalls        []core.ToolCall         `json:"tool_calls,omitempty"`
	SourceUses       []core.SourceUse        `json:"source_uses,omitempty"`
	Approvals        []core.ApprovalArtifact `json:"approvals,omitempty"`
	StateChanges     []core.StateChange      `json:"state_changes,omitempty"`
	MemoryOperations []core.MemoryOperation  `json:"memory_operations,omitempty"`
}

type StateAdapterInput struct {
	Request  core.Request  `json:"request"`
	Response core.Response `json:"response"`
}

type SuiteInput struct {
	Config core.Config         `json:"config"`
	Suite  core.PluginSuite    `json:"suite"`
	Plugin core.PluginManifest `json:"plugin"`
}

func ApplyStateAdapters(ctx context.Context, req core.Request, resp core.Response, manifests []core.PluginManifest) (core.Response, error) {
	for _, manifest := range manifests {
		for _, adapter := range manifest.StateAdapters {
			delta, err := runStateAdapter(ctx, adapter, StateAdapterInput{Request: req, Response: resp})
			if err != nil {
				return resp, err
			}
			resp.Normalized.ToolCalls = append(resp.Normalized.ToolCalls, delta.ToolCalls...)
			resp.Normalized.SourceUses = append(resp.Normalized.SourceUses, delta.SourceUses...)
			resp.Normalized.Approvals = append(resp.Normalized.Approvals, delta.Approvals...)
			resp.Normalized.StateChanges = append(resp.Normalized.StateChanges, delta.StateChanges...)
			resp.Normalized.MemoryOperations = append(resp.Normalized.MemoryOperations, delta.MemoryOperations...)
		}
	}
	return resp, nil
}

func RunPluginSuite(ctx context.Context, manifest core.PluginManifest, suite core.PluginSuite, cfg core.Config) (core.SuiteResult, error) {
	input := SuiteInput{
		Config: cfg,
		Suite:  suite,
		Plugin: manifest,
	}
	var result core.SuiteResult
	if err := runJSONCommand(ctx, suite.Command, suite.Args, suite.Env, suite.TimeoutMS, input, &result); err != nil {
		return core.SuiteResult{}, err
	}
	if strings.TrimSpace(result.Name) == "" {
		result.Name = suite.Name
	}
	return result, nil
}

func runStateAdapter(ctx context.Context, adapter core.PluginStateAdapter, input StateAdapterInput) (StateAdapterDelta, error) {
	var delta StateAdapterDelta
	if err := runJSONCommand(ctx, adapter.Command, adapter.Args, adapter.Env, adapter.TimeoutMS, input, &delta); err != nil {
		return StateAdapterDelta{}, fmt.Errorf("plugin state adapter %s: %w", adapter.Name, err)
	}
	return delta, nil
}

func runJSONCommand(ctx context.Context, command string, args []string, env map[string]string, timeoutMS int, input any, output any) error {
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(cmdCtx, command, args...)
	cmd.Stdin = bytes.NewReader(data)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append([]string(nil), os.Environ()...)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	if err := json.Unmarshal(stdout.Bytes(), output); err != nil {
		return fmt.Errorf("decode plugin output: %w", err)
	}
	return nil
}
