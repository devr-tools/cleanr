package snapshots

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return decodeFile(data, path)
}

func WriteFile(path string, snapshot File) error {
	data, err := encodeFile(snapshot, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func decodeFile(data []byte, path string) (File, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return File{}, fmt.Errorf("decode snapshots: %w", err)
		}
		raw, err := json.Marshal(normalizeYAMLValue(generic))
		if err != nil {
			return File{}, fmt.Errorf("decode snapshots: %w", err)
		}
		var snapshot File
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return File{}, fmt.Errorf("decode snapshots: %w", err)
		}
		return snapshot, nil
	}

	var snapshot File
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return File{}, fmt.Errorf("decode snapshots: %w", err)
	}
	return snapshot, nil
}

func encodeFile(snapshot File, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return nil, fmt.Errorf("encode snapshots: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode snapshots: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode snapshots: %w", err)
		}
		return data, nil
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode snapshots: %w", err)
	}
	return data, nil
}

func isYAMLPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
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
