package trends

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (HistoryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HistoryFile{}, err
	}
	return LoadData(data, path)
}

func LoadData(data []byte, path string) (HistoryFile, error) {
	return decodeFile(data, path)
}

func WriteFile(path string, history HistoryFile) error {
	data, err := encodeFile(history, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o644)
}

// writeFileAtomic writes data to a temp file in the same directory as path and
// renames it over the target so an interrupt can never leave a truncated or
// partially-written state file behind.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func decodeFile(data []byte, path string) (HistoryFile, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return HistoryFile{}, fmt.Errorf("decode trend history: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return HistoryFile{}, fmt.Errorf("decode trend history: %w", err)
		}
		var history HistoryFile
		if err := json.Unmarshal(raw, &history); err != nil {
			return HistoryFile{}, fmt.Errorf("decode trend history: %w", err)
		}
		return history, nil
	}
	var history HistoryFile
	if err := json.Unmarshal(data, &history); err != nil {
		return HistoryFile{}, fmt.Errorf("decode trend history: %w", err)
	}
	return history, nil
}

func encodeFile(history HistoryFile, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(history)
		if err != nil {
			return nil, fmt.Errorf("encode trend history: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode trend history: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode trend history: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode trend history: %w", err)
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
