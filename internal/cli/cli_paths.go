package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

func resolveConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	candidates := []string{"cleanr.json", "cleanr.yaml", "cleanr.yml"}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no config file found; expected one of %s in %s", joinCandidates(candidates), mustGetwd())
}

func joinCandidates(paths []string) string {
	quoted := make([]string, 0, len(paths))
	for _, path := range paths {
		quoted = append(quoted, filepath.Base(path))
	}
	return strings.Join(quoted, ", ")
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func resolveConfigRelativePath(configPath, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(configPath), path)
}

func resolveTrendPath(configPath, explicitTrendPath string) (string, error) {
	if strings.TrimSpace(explicitTrendPath) != "" {
		return explicitTrendPath, nil
	}
	resolvedConfigPath, err := resolveConfigPath(configPath)
	if err != nil {
		return "", err
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		return "", err
	}
	trendPath := resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.TrendFile)
	if strings.TrimSpace(trendPath) == "" {
		return "", fmt.Errorf("no trend file configured; set reporting.trend_file or pass -trend-file")
	}
	return trendPath, nil
}

func writeJSON(w io.Writer, value any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return 2
	}
	return 0
}
