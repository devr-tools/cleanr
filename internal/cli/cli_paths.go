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

const configProfileEnvName = "CLEANR_PROFILE"

var defaultConfigCandidates = []string{"cleanr.json", "cleanr.yaml", "cleanr.yml"}

func resolveConfigPath(configPath, profile string) (string, error) {
	if strings.TrimSpace(configPath) != "" {
		return configPath, nil
	}

	resolvedProfile, err := resolveConfigProfile(profile)
	if err != nil {
		return "", err
	}
	if resolvedProfile != "" {
		return resolveStagedConfigPath(resolvedProfile)
	}

	for _, candidate := range defaultConfigCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	err = fmt.Errorf("no config file found; expected one of %s in %s", joinCandidates(defaultConfigCandidates), mustGetwd())
	if hasStagedConfigFiles() {
		return "", fmt.Errorf("%w; found staged configs under .cleanr, rerun with -profile pr|main|release or set %s", err, configProfileEnvName)
	}
	return "", err
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

func resolveConfigProfile(profile string) (string, error) {
	profile = strings.ToLower(strings.TrimSpace(firstNonEmpty(profile, os.Getenv(configProfileEnvName))))
	switch profile {
	case "":
		return "", nil
	case "pr", "main", "release":
		return profile, nil
	default:
		return "", fmt.Errorf("unsupported profile %q; expected pr, main, or release", profile)
	}
}

func resolveStagedConfigPath(profile string) (string, error) {
	candidates := []string{
		filepath.Join(".cleanr", profile+".json"),
		filepath.Join(".cleanr", profile+".yaml"),
		filepath.Join(".cleanr", profile+".yml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no config file found for profile %q; expected one of %s in %s", profile, strings.Join(candidates, ", "), mustGetwd())
}

func hasStagedConfigFiles() bool {
	for _, profile := range []string{"pr", "main", "release"} {
		for _, candidate := range []string{
			filepath.Join(".cleanr", profile+".json"),
			filepath.Join(".cleanr", profile+".yaml"),
			filepath.Join(".cleanr", profile+".yml"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return true
			}
		}
	}
	return false
}

func resolveConfigRelativePath(configPath, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(configPath), path)
}

func resolveTrendPath(configPath, profile, explicitTrendPath string) (string, error) {
	if strings.TrimSpace(explicitTrendPath) != "" {
		return explicitTrendPath, nil
	}
	resolvedConfigPath, err := resolveConfigPath(configPath, profile)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
