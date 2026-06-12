package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
	"gopkg.in/yaml.v3"
)

func LoadConfigFile(path string) (core.Config, error) {
	cfg, err := loadConfigFile(path, nil)
	if err != nil {
		return core.Config{}, err
	}
	applyDefaults(&cfg)
	if err := ValidateConfig(cfg); err != nil {
		return core.Config{}, err
	}

	return cfg, nil
}

func LoadConfigData(data []byte, format string) (core.Config, error) {
	cfg, err := decodeConfigWithPolicyPacks(data, syntheticPath(format), ".", nil)
	if err != nil {
		return core.Config{}, err
	}
	applyDefaults(&cfg)
	if err := ValidateConfig(cfg); err != nil {
		return core.Config{}, err
	}
	return cfg, nil
}

func WriteConfigFile(path string, cfg core.Config) error {
	data, err := encodeConfig(cfg, path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func MarshalConfig(cfg core.Config, format string) ([]byte, error) {
	data, err := encodeConfig(cfg, syntheticPath(format))
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodeConfig(data []byte, path string) (core.Config, error) {
	return decodeConfigWithPolicyPacks(data, path, filepath.Dir(path), nil)
}

func encodeConfig(cfg core.Config, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("encode config: %w", err)
		}

		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode config: %w", err)
		}

		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode config: %w", err)
		}
		return data, nil
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return data, nil
}

func decodeYAMLConfig(data []byte) (core.Config, error) {
	var generic any
	if err := yaml.Unmarshal(data, &generic); err != nil {
		return core.Config{}, fmt.Errorf("decode config: %w", err)
	}

	normalized := normalizeYAMLValue(generic)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return core.Config{}, fmt.Errorf("decode config: %w", err)
	}

	var cfg core.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return core.Config{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func loadConfigFile(path string, seen map[string]struct{}) (core.Config, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		abs, err := filepath.Abs(resolved)
		if err == nil {
			resolved = abs
		}
	}
	if seen == nil {
		seen = map[string]struct{}{}
	}
	if _, ok := seen[resolved]; ok {
		return core.Config{}, fmt.Errorf("decode config: policy pack cycle detected at %s", resolved)
	}
	seen[resolved] = struct{}{}
	defer delete(seen, resolved)

	data, err := os.ReadFile(resolved)
	if err != nil {
		return core.Config{}, err
	}
	return decodeConfigWithPolicyPacks(data, resolved, filepath.Dir(resolved), seen)
}

func decodeConfigWithPolicyPacks(data []byte, path, baseDir string, seen map[string]struct{}) (core.Config, error) {
	generic, err := decodeGenericConfig(data, path)
	if err != nil {
		return core.Config{}, err
	}
	pluginPaths := stringList(generic["plugins"])
	pluginManifests, pluginDefaults, err := loadPluginManifests(baseDir, pluginPaths, seen)
	if err != nil {
		return core.Config{}, err
	}
	merged := cloneMap(generic)
	if len(pluginDefaults) > 0 {
		merged = applyDefaultsGenerics(merged, pluginDefaults)
	}
	merged, err = applyPolicyPackGenerics(merged, baseDir, seen)
	if err != nil {
		return core.Config{}, err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return core.Config{}, fmt.Errorf("decode config: %w", err)
	}
	var cfg core.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return core.Config{}, fmt.Errorf("decode config: %w", err)
	}
	cfg.ResolvedPlugins = pluginManifests
	return cfg, nil
}

func decodeGenericConfig(data []byte, path string) (map[string]any, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return nil, fmt.Errorf("decode config: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		if mapped, ok := normalized.(map[string]any); ok {
			return mapped, nil
		}
		return map[string]any{}, nil
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return generic, nil
}

func applyPolicyPackGenerics(base map[string]any, baseDir string, seen map[string]struct{}) (map[string]any, error) {
	packs := stringList(base["policy_packs"])
	delete(base, "policy_packs")
	defaults := map[string]any{}
	for _, pack := range packs {
		if strings.TrimSpace(pack) == "" {
			continue
		}
		packPath := resolveRelativePath(baseDir, pack)
		packGeneric, err := loadPackGenericFile(packPath, seen)
		if err != nil {
			return nil, err
		}
		delete(packGeneric, "policy_packs")
		delete(packGeneric, "plugins")
		defaults = overlayConfig(defaults, packGeneric)
	}
	merged := applyDefaultsGenerics(base, defaults)
	if len(packs) > 0 {
		merged["policy_packs"] = packs
	}
	return merged, nil
}

func applyDefaultsGenerics(base, defaults map[string]any) map[string]any {
	if len(defaults) == 0 {
		return cloneMap(base)
	}
	return overlayConfig(defaults, base)
}

func overlayConfig(base, override map[string]any) map[string]any {
	out := cloneMap(base)
	for key, overrideValue := range override {
		baseValue, ok := out[key]
		if !ok {
			out[key] = overrideValue
			continue
		}
		baseMap, baseIsMap := baseValue.(map[string]any)
		overrideMap, overrideIsMap := overrideValue.(map[string]any)
		if baseIsMap && overrideIsMap {
			out[key] = overlayConfig(baseMap, overrideMap)
			continue
		}
		out[key] = overrideValue
	}
	return out
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	default:
		return nil
	}
}

func resolveRelativePath(baseDir, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) {
		return rel
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}
	return filepath.Join(baseDir, rel)
}

func loadPackGenericFile(path string, seen map[string]struct{}) (map[string]any, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		abs, err := filepath.Abs(resolved)
		if err == nil {
			resolved = abs
		}
	}
	if seen == nil {
		seen = map[string]struct{}{}
	}
	if _, ok := seen[resolved]; ok {
		return nil, fmt.Errorf("decode config: policy pack cycle detected at %s", resolved)
	}
	seen[resolved] = struct{}{}
	defer delete(seen, resolved)

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	generic, err := decodeGenericConfig(data, resolved)
	if err != nil {
		return nil, err
	}
	return applyPolicyPackGenerics(generic, filepath.Dir(resolved), seen)
}

func loadPluginManifests(baseDir string, pluginPaths []string, seen map[string]struct{}) ([]core.PluginManifest, map[string]any, error) {
	if len(pluginPaths) == 0 {
		return nil, nil, nil
	}
	manifests := make([]core.PluginManifest, 0, len(pluginPaths))
	defaults := map[string]any{}
	for _, pluginPath := range pluginPaths {
		manifest, pluginDefaults, err := loadPluginManifestFile(resolveRelativePath(baseDir, pluginPath), seen)
		if err != nil {
			return nil, nil, err
		}
		manifests = append(manifests, manifest)
		defaults = overlayConfig(defaults, pluginDefaults)
	}
	return manifests, defaults, nil
}

func loadPluginManifestFile(path string, seen map[string]struct{}) (core.PluginManifest, map[string]any, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		abs, err := filepath.Abs(resolved)
		if err == nil {
			resolved = abs
		}
	}
	manifest, err := decodePluginManifestFile(resolved)
	if err != nil {
		return core.PluginManifest{}, nil, err
	}
	defaults, err := loadPluginPolicyPackDefaults(manifest.PolicyPacks, filepath.Dir(resolved), seen)
	if err != nil {
		return core.PluginManifest{}, nil, err
	}
	return manifest, defaults, nil
}

func decodePluginManifestFile(path string) (core.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return core.PluginManifest{}, err
	}
	generic, err := decodeGenericConfig(data, path)
	if err != nil {
		return core.PluginManifest{}, err
	}
	manifest, err := decodePluginManifest(generic)
	if err != nil {
		return core.PluginManifest{}, err
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if err := validatePluginManifest(manifest, path); err != nil {
		return core.PluginManifest{}, err
	}
	return manifest, nil
}

func decodePluginManifest(generic map[string]any) (core.PluginManifest, error) {
	raw, err := json.Marshal(generic)
	if err != nil {
		return core.PluginManifest{}, fmt.Errorf("decode config: %w", err)
	}
	var manifest core.PluginManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return core.PluginManifest{}, fmt.Errorf("decode config: %w", err)
	}
	return manifest, nil
}

func loadPluginPolicyPackDefaults(packs []string, baseDir string, seen map[string]struct{}) (map[string]any, error) {
	defaults := map[string]any{}
	var err error
	for _, pack := range packs {
		defaults, err = overlayPluginPolicyPack(defaults, pack, baseDir, seen)
		if err != nil {
			return nil, err
		}
	}
	return defaults, nil
}

func overlayPluginPolicyPack(defaults map[string]any, pack, baseDir string, seen map[string]struct{}) (map[string]any, error) {
	packGeneric, err := loadPackGenericFile(resolveRelativePath(baseDir, pack), seen)
	if err != nil {
		return nil, err
	}
	delete(packGeneric, "plugins")
	return overlayConfig(defaults, packGeneric), nil
}

func validatePluginManifest(manifest core.PluginManifest, path string) error {
	err := validateNamedPluginCommands(path, "suite", manifest.Suites, func(item core.PluginSuite) (string, string) {
		return item.Name, item.Command
	})
	if err != nil {
		return err
	}
	err = validateNamedPluginCommands(path, "state_adapter", manifest.StateAdapters, func(item core.PluginStateAdapter) (string, string) {
		return item.Name, item.Command
	})
	if err != nil {
		return err
	}
	return validateNamedPluginCommands(path, "probe", manifest.Probes, func(item core.PluginProbe) (string, string) {
		return item.Name, item.Command
	})
}

func validateNamedPluginCommands[T any](path, kind string, items []T, fields func(T) (string, string)) error {
	for i, item := range items {
		name, command := fields(item)
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("decode config: plugin %s %s[%d] is missing name", path, kind, i)
		}
		if strings.TrimSpace(command) == "" {
			return fmt.Errorf("decode config: plugin %s %s[%d] is missing command", path, kind, i)
		}
	}
	return nil
}

func normalizeYAMLValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeYAMLValue(value)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeYAMLValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeYAMLValue(value)
		}
		return out
	default:
		return v
	}
}

func isYAMLPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func syntheticPath(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		return "inline.yaml"
	default:
		return "inline.json"
	}
}
