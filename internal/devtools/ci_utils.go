package devtools

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func normalizeBaseBranchName(baseRef string) string {
	trimmed := strings.TrimSpace(baseRef)
	trimmed = strings.TrimPrefix(trimmed, "refs/remotes/")
	trimmed = strings.TrimPrefix(trimmed, "refs/heads/")
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func parseCoverageTotal(report string) (float64, error) {
	for _, line := range splitNonEmptyLines(report) {
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}
		value := strings.TrimSuffix(fields[len(fields)-1], "%")
		total, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("parse coverage total %q: %w", value, err)
		}
		return total, nil
	}
	return 0, fmt.Errorf("coverage report missing total line")
}

func splitNonEmptyLines(input string) []string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func filterMatching(lines []string, pattern *regexp.Regexp) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if pattern.MatchString(line) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func firstLines(input string, limit int) string {
	lines := splitNonEmptyLines(input)
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func resolveCIString(explicit, envKey, fallback string) string {
	value := strings.TrimSpace(explicit)
	if value != "" {
		return value
	}
	if envKey != "" {
		value = strings.TrimSpace(os.Getenv(envKey))
		if value != "" {
			return value
		}
	}
	return fallback
}

func resolveCICoverageThreshold(explicit float64) float64 {
	if explicit > 0 {
		return explicit
	}
	raw := strings.TrimSpace(os.Getenv("MIN_INTERNAL_COVERAGE"))
	if raw == "" {
		return defaultCIMinCoverage
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return defaultCIMinCoverage
	}
	return value
}

func resolveCIMaxFileCodeLines(explicit int) int {
	if explicit > 0 {
		return explicit
	}
	raw := strings.TrimSpace(os.Getenv("MAX_GO_FILE_CODE_LINES"))
	if raw == "" {
		return defaultCIMaxFileCodeLines
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultCIMaxFileCodeLines
	}
	return value
}

func resolveCIMaxFunctionComplexity(explicit int) int {
	if explicit > 0 {
		return explicit
	}
	raw := strings.TrimSpace(os.Getenv("MAX_FUNCTION_COMPLEXITY"))
	if raw == "" {
		return defaultCIMaxFunctionComplexity
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultCIMaxFunctionComplexity
	}
	return value
}

func shouldFallbackToPrebuiltGoTool(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "invalid go version") ||
		strings.Contains(message, "unknown block type: ignore") ||
		strings.Contains(message, "unknown directive: ignore")
}
