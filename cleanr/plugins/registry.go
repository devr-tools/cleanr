package plugins

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type Backend string

const (
	BackendCommand Backend = "command"
	BackendWASM    Backend = "wasm"
)

type Registry struct {
	manifests []core.PluginManifest
}

type Entry struct {
	Plugin     core.PluginManifest
	Kind       string
	ProbeKind  string
	Name       string
	Command    string
	Args       []string
	Env        map[string]string
	TimeoutMS  int
	Runtime    core.PluginRuntimeConfig
	Resolved   string
	WorkingDir string
}

func NewRegistry(manifests []core.PluginManifest) Registry {
	copied := make([]core.PluginManifest, len(manifests))
	copy(copied, manifests)
	return Registry{manifests: copied}
}

func (r Registry) Manifests() []core.PluginManifest {
	copied := make([]core.PluginManifest, len(r.manifests))
	copy(copied, r.manifests)
	return copied
}

func (r Registry) HasStateAdapters() bool {
	for _, manifest := range r.manifests {
		if len(manifest.StateAdapters) > 0 || len(manifest.Probes) > 0 {
			return true
		}
	}
	return false
}

func (r Registry) Suites() []Entry {
	var entries []Entry
	for _, manifest := range r.manifests {
		for _, suite := range manifest.Suites {
			entries = append(entries, buildEntry(manifest, "suite", suite.Name, suite.Command, suite.Args, suite.Env, suite.TimeoutMS, suite.Runtime))
		}
	}
	return entries
}

func (r Registry) StateAdapters() []Entry {
	var entries []Entry
	for _, manifest := range r.manifests {
		for _, adapter := range manifest.StateAdapters {
			entries = append(entries, buildEntry(manifest, "state_adapter", adapter.Name, adapter.Command, adapter.Args, adapter.Env, adapter.TimeoutMS, adapter.Runtime))
		}
	}
	return entries
}

func (r Registry) Probes() []Entry {
	var entries []Entry
	for _, manifest := range r.manifests {
		for _, probe := range manifest.Probes {
			entry := buildEntry(manifest, "probe", probe.Name, probe.Command, probe.Args, probe.Env, probe.TimeoutMS, probe.Runtime)
			entry.ProbeKind = probe.Kind
			entries = append(entries, entry)
		}
	}
	return entries
}

func buildEntry(manifest core.PluginManifest, kind, name, command string, args []string, env map[string]string, timeoutMS int, runtime core.PluginRuntimeConfig) Entry {
	effectiveRuntime := runtime
	if strings.TrimSpace(effectiveRuntime.Backend) == "" {
		effectiveRuntime = manifest.Runtime
	}
	resolved := resolveEntryPath(manifest.BaseDir, command, effectiveRuntime)
	return Entry{
		Plugin:     manifest,
		Kind:       kind,
		Name:       name,
		Command:    command,
		Args:       append([]string(nil), args...),
		Env:        cloneStringMap(env),
		TimeoutMS:  timeoutMS,
		Runtime:    effectiveRuntime,
		Resolved:   resolved,
		WorkingDir: manifest.BaseDir,
	}
}

func resolveEntryPath(baseDir, command string, runtime core.PluginRuntimeConfig) string {
	command = strings.TrimSpace(command)
	if command == "" || filepath.IsAbs(command) {
		return command
	}
	backend := backendFor(command, runtime)
	if backend == BackendWASM || strings.HasPrefix(command, ".") || strings.Contains(command, string(filepath.Separator)) {
		if strings.TrimSpace(baseDir) == "" {
			return command
		}
		return filepath.Join(baseDir, command)
	}
	return command
}

func backendFor(command string, runtime core.PluginRuntimeConfig) Backend {
	switch strings.ToLower(strings.TrimSpace(runtime.Backend)) {
	case "wasm":
		return BackendWASM
	case "command", "":
		if strings.EqualFold(filepath.Ext(strings.TrimSpace(command)), ".wasm") {
			return BackendWASM
		}
		return BackendCommand
	default:
		return BackendCommand
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
