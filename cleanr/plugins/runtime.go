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

	"github.com/devr-tools/cleanr/cleanr/core"
)

type StateAdapterDelta struct {
	ToolCalls         []core.ToolCall              `json:"tool_calls,omitempty"`
	SourceUses        []core.SourceUse             `json:"source_uses,omitempty"`
	Approvals         []core.ApprovalArtifact      `json:"approvals,omitempty"`
	StateChanges      []core.StateChange           `json:"state_changes,omitempty"`
	MemoryOperations  []core.MemoryOperation       `json:"memory_operations,omitempty"`
	DBObservations    []core.DBProbeObservation    `json:"db_observations,omitempty"`
	QueueObservations []core.QueueProbeObservation `json:"queue_observations,omitempty"`
}

type StateAdapterInput struct {
	Scenario core.Scenario `json:"scenario"`
	Request  core.Request  `json:"request"`
	Response core.Response `json:"response"`
}

type SuiteInput struct {
	Config core.Config         `json:"config"`
	Suite  core.PluginSuite    `json:"suite"`
	Plugin core.PluginManifest `json:"plugin"`
}

type jsonCommandSpec struct {
	command   string
	args      []string
	env       map[string]string
	timeoutMS int
}

func ApplyStateAdapters(ctx context.Context, req core.Request, resp core.Response, manifests []core.PluginManifest) (core.Response, error) {
	for _, manifest := range manifests {
		input := StateAdapterInput{
			Scenario: req.Scenario,
			Request:  req,
			Response: resp,
		}
		for _, adapter := range manifest.StateAdapters {
			delta, err := runStateAdapter(ctx, adapter, input)
			if err != nil {
				return resp, err
			}
			resp = applyStateAdapterDelta(resp, delta)
		}
		for _, probe := range manifest.Probes {
			delta, err := runProbe(ctx, probe, input)
			if err != nil {
				return resp, err
			}
			resp = applyStateAdapterDelta(resp, delta)
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
	if err := runJSONCommand(ctx, jsonCommandSpec{
		command:   suite.Command,
		args:      suite.Args,
		env:       suite.Env,
		timeoutMS: suite.TimeoutMS,
	}, input, &result); err != nil {
		return core.SuiteResult{}, err
	}
	if strings.TrimSpace(result.Name) == "" {
		result.Name = suite.Name
	}
	return result, nil
}

func runStateAdapter(ctx context.Context, adapter core.PluginStateAdapter, input StateAdapterInput) (StateAdapterDelta, error) {
	var delta StateAdapterDelta
	if err := runJSONCommand(ctx, jsonCommandSpec{
		command:   adapter.Command,
		args:      adapter.Args,
		env:       adapter.Env,
		timeoutMS: adapter.TimeoutMS,
	}, input, &delta); err != nil {
		return StateAdapterDelta{}, fmt.Errorf("plugin state adapter %s: %w", adapter.Name, err)
	}
	return delta, nil
}

func runProbe(ctx context.Context, probe core.PluginProbe, input StateAdapterInput) (StateAdapterDelta, error) {
	var delta StateAdapterDelta
	if err := runJSONCommand(ctx, jsonCommandSpec{
		command:   probe.Command,
		args:      probe.Args,
		env:       probe.Env,
		timeoutMS: probe.TimeoutMS,
	}, input, &delta); err != nil {
		return StateAdapterDelta{}, fmt.Errorf("plugin probe %s: %w", probe.Name, err)
	}
	delta.StateChanges = append(delta.StateChanges, normalizeDBProbeObservations(probe, delta.DBObservations)...)
	delta.StateChanges = append(delta.StateChanges, normalizeQueueProbeObservations(probe, delta.QueueObservations)...)
	return delta, nil
}

func applyStateAdapterDelta(resp core.Response, delta StateAdapterDelta) core.Response {
	resp.Normalized.ToolCalls = append(resp.Normalized.ToolCalls, delta.ToolCalls...)
	resp.Normalized.SourceUses = append(resp.Normalized.SourceUses, delta.SourceUses...)
	resp.Normalized.Approvals = append(resp.Normalized.Approvals, delta.Approvals...)
	resp.Normalized.StateChanges = append(resp.Normalized.StateChanges, delta.StateChanges...)
	resp.Normalized.MemoryOperations = append(resp.Normalized.MemoryOperations, delta.MemoryOperations...)
	return resp
}

func normalizeDBProbeObservations(probe core.PluginProbe, observations []core.DBProbeObservation) []core.StateChange {
	out := make([]core.StateChange, 0, len(observations))
	for _, observation := range observations {
		raw := cloneRawMap(observation.Raw)
		raw["probe"] = probe.Name
		raw["probe_kind"] = probeKindValue(probe, "db")
		if observation.Engine != "" {
			raw["engine"] = observation.Engine
		}
		if observation.Database != "" {
			raw["database"] = observation.Database
		}
		if observation.Table != "" {
			raw["table"] = observation.Table
		}
		if observation.Count != 0 {
			raw["count"] = observation.Count
		}
		out = append(out, core.StateChange{
			Kind:    "db",
			Target:  dbProbeTarget(observation),
			Action:  defaultString(observation.Operation, "observe"),
			Status:  observation.Status,
			Summary: defaultString(observation.Summary, renderDBProbeSummary(observation)),
			Raw:     raw,
		})
	}
	return out
}

func normalizeQueueProbeObservations(probe core.PluginProbe, observations []core.QueueProbeObservation) []core.StateChange {
	out := make([]core.StateChange, 0, len(observations))
	for _, observation := range observations {
		raw := cloneRawMap(observation.Raw)
		raw["probe"] = probe.Name
		raw["probe_kind"] = probeKindValue(probe, "queue")
		if observation.Provider != "" {
			raw["provider"] = observation.Provider
		}
		if observation.Queue != "" {
			raw["queue"] = observation.Queue
		}
		if observation.Topic != "" {
			raw["topic"] = observation.Topic
		}
		if observation.MessageID != "" {
			raw["message_id"] = observation.MessageID
		}
		if observation.Depth != 0 {
			raw["depth"] = observation.Depth
		}
		out = append(out, core.StateChange{
			Kind:    "queue",
			Target:  queueProbeTarget(observation),
			Action:  defaultString(observation.Operation, "observe"),
			Status:  observation.Status,
			Summary: defaultString(observation.Summary, renderQueueProbeSummary(observation)),
			Raw:     raw,
		})
	}
	return out
}

func dbProbeTarget(observation core.DBProbeObservation) string {
	switch {
	case observation.Database != "" && observation.Table != "":
		return observation.Database + "." + observation.Table
	case observation.Table != "":
		return observation.Table
	default:
		return observation.Database
	}
}

func queueProbeTarget(observation core.QueueProbeObservation) string {
	switch {
	case observation.Queue != "" && observation.Topic != "":
		return observation.Queue + ":" + observation.Topic
	case observation.Queue != "":
		return observation.Queue
	default:
		return observation.Topic
	}
}

func renderDBProbeSummary(observation core.DBProbeObservation) string {
	target := dbProbeTarget(observation)
	if target == "" {
		target = "database"
	}
	if observation.Count > 0 {
		return fmt.Sprintf("%s %s count=%d", target, defaultString(observation.Operation, "observe"), observation.Count)
	}
	return fmt.Sprintf("%s %s", target, defaultString(observation.Operation, "observe"))
}

func renderQueueProbeSummary(observation core.QueueProbeObservation) string {
	target := queueProbeTarget(observation)
	if target == "" {
		target = "queue"
	}
	if observation.Depth > 0 {
		return fmt.Sprintf("%s %s depth=%d", target, defaultString(observation.Operation, "observe"), observation.Depth)
	}
	return fmt.Sprintf("%s %s", target, defaultString(observation.Operation, "observe"))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func probeKindValue(probe core.PluginProbe, fallback string) string {
	if strings.TrimSpace(probe.Kind) == "" {
		return fallback
	}
	return strings.TrimSpace(probe.Kind)
}

func cloneRawMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func runJSONCommand(ctx context.Context, spec jsonCommandSpec, input any, output any) error {
	timeout := time.Duration(spec.timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(cmdCtx, spec.command, spec.args...)
	cmd.Stdin = bytes.NewReader(data)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append([]string(nil), os.Environ()...)
	for key, value := range spec.env {
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
