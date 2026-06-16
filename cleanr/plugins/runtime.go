package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func ApplyStateAdapters(ctx context.Context, req core.Request, resp core.Response, manifests []core.PluginManifest) (core.Response, error) {
	registry := NewRegistry(manifests)
	for _, adapter := range registry.StateAdapters() {
		input := StateAdapterInput{Scenario: req.Scenario, Request: req, Response: resp}
		delta, err := runStateAdapter(ctx, adapter, input)
		if err != nil {
			return resp, err
		}
		resp = applyStateAdapterDelta(resp, delta)
	}
	for _, probe := range registry.Probes() {
		input := StateAdapterInput{Scenario: req.Scenario, Request: req, Response: resp}
		delta, err := runProbe(ctx, probe, input)
		if err != nil {
			return resp, err
		}
		resp = applyStateAdapterDelta(resp, delta)
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
	entry := buildEntry(manifest, "suite", suite.Name, suite.Command, suite.Args, suite.Env, suite.TimeoutMS, suite.Runtime)
	if err := runJSONEntry(ctx, entry, input, &result); err != nil {
		return core.SuiteResult{}, err
	}
	if strings.TrimSpace(result.Name) == "" {
		result.Name = suite.Name
	}
	return result, nil
}

func runStateAdapter(ctx context.Context, adapter Entry, input StateAdapterInput) (StateAdapterDelta, error) {
	var delta StateAdapterDelta
	if err := runJSONEntry(ctx, adapter, input, &delta); err != nil {
		return StateAdapterDelta{}, fmt.Errorf("plugin state adapter %s: %w", adapter.Name, err)
	}
	return delta, nil
}

func runProbe(ctx context.Context, probe Entry, input StateAdapterInput) (StateAdapterDelta, error) {
	var delta StateAdapterDelta
	if err := runJSONEntry(ctx, probe, input, &delta); err != nil {
		return StateAdapterDelta{}, fmt.Errorf("plugin probe %s: %w", probe.Name, err)
	}
	delta.StateChanges = append(delta.StateChanges, normalizeDBProbeObservations(probeDescriptor(probe), delta.DBObservations)...)
	delta.StateChanges = append(delta.StateChanges, normalizeQueueProbeObservations(probeDescriptor(probe), delta.QueueObservations)...)
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

func runJSONEntry(ctx context.Context, entry Entry, input any, output any) error {
	timeout := time.Duration(entry.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	stdout, err := executeEntry(cmdCtx, entry, data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(stdout.Bytes(), output); err != nil {
		return fmt.Errorf("decode plugin output: %w", err)
	}
	return nil
}

func executeEntry(ctx context.Context, entry Entry, input []byte) (bytes.Buffer, error) {
	switch backendFor(entry.Command, entry.Runtime) {
	case BackendWASM:
		return runWASMModule(ctx, entry, input)
	default:
		return runCommand(ctx, entry, input)
	}
}

func runCommand(ctx context.Context, entry Entry, input []byte) (bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, entry.Resolved, entry.Args...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = commandWorkingDir(entry)
	cmd.Env = buildEntryEnv(entry)
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return bytes.Buffer{}, fmt.Errorf("%s", msg)
	}
	return stdout, nil
}

func buildEntryEnv(entry Entry) []string {
	env := append([]string(nil), os.Environ()...)
	for key, value := range entry.Env {
		env = append(env, key+"="+value)
	}
	if strings.TrimSpace(entry.Plugin.BaseDir) != "" {
		env = append(env, "CLEANR_PLUGIN_DIR="+entry.Plugin.BaseDir)
	}
	if strings.TrimSpace(entry.Plugin.Source) != "" {
		env = append(env, "CLEANR_PLUGIN_SOURCE="+entry.Plugin.Source)
	}
	if strings.TrimSpace(entry.Plugin.Name) != "" {
		env = append(env, "CLEANR_PLUGIN_NAME="+entry.Plugin.Name)
	}
	return env
}

func commandWorkingDir(entry Entry) string {
	if strings.TrimSpace(entry.WorkingDir) != "" {
		return entry.WorkingDir
	}
	if strings.TrimSpace(entry.Resolved) != "" && filepath.IsAbs(entry.Resolved) {
		return filepath.Dir(entry.Resolved)
	}
	return ""
}

func probeDescriptor(entry Entry) core.PluginProbe {
	return core.PluginProbe{
		Name:    entry.Name,
		Kind:    entry.ProbeKind,
		Command: entry.Command,
		Args:    append([]string(nil), entry.Args...),
		Env:     cloneStringMap(entry.Env),
		Runtime: entry.Runtime,
	}
}
