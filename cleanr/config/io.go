package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cleanr/cleanr/core"
	"gopkg.in/yaml.v3"
)

func LoadConfigFile(path string) (core.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return core.Config{}, err
	}

	cfg, err := decodeConfig(data, path)
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

func decodeConfig(data []byte, path string) (core.Config, error) {
	if isYAMLPath(path) {
		return decodeYAMLConfig(data)
	}

	var cfg core.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return core.Config{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
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
