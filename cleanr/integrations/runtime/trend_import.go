package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	trendspkg "github.com/devr-tools/cleanr/cleanr/trends"
	"gopkg.in/yaml.v3"
)

func loadExternalTrendSource(ctx context.Context, source core.TrendSourceConfig, baseDir string) (trendspkg.HistoryFile, error) {
	data, origin, err := readTrendSourceBytes(ctx, source, baseDir)
	if err != nil {
		return trendspkg.HistoryFile{}, err
	}
	return importTrendSourceData(data, source, origin)
}

func readTrendSourceBytes(ctx context.Context, source core.TrendSourceConfig, baseDir string) ([]byte, string, error) {
	if path := strings.TrimSpace(source.Path); path != "" {
		resolved := resolveRelativePath(baseDir, path)
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, "", fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
		}
		return data, resolved, nil
	}
	client := &http.Client{Timeout: time.Duration(source.TimeoutMS) * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(source.URL), nil)
	if err != nil {
		return nil, "", fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
	}
	applyAuth(req.Header, source.APIKeyEnv, source.URL)
	applyHeaders(req.Header, source.Headers)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("load trend source %s: %s", displayName(source.Name, source.Type), compactHTTPError(resp.StatusCode, data))
	}
	return data, strings.TrimSpace(source.URL), nil
}

func importTrendSourceData(data []byte, source core.TrendSourceConfig, origin string) (trendspkg.HistoryFile, error) {
	if history, err := trendspkg.LoadData(data, origin); err == nil {
		if looksLikeNativeHistory(history) {
			return history, nil
		}
	}
	payload, err := decodeArbitraryPayload(data)
	if err != nil {
		return trendspkg.HistoryFile{}, fmt.Errorf("load trend source %s: %w", displayName(source.Name, source.Type), err)
	}
	runs := importedRunsForSource(payload, source.Type)
	if len(runs) == 0 {
		return trendspkg.HistoryFile{}, fmt.Errorf("load trend source %s: no importable runs found", displayName(source.Name, source.Type))
	}
	history := trendspkg.NewHistory(importTargetName(source))
	for _, run := range runs {
		history.Runs = append(history.Runs, run)
	}
	sort.Slice(history.Runs, func(i, j int) bool { return history.Runs[i].GeneratedAt.Before(history.Runs[j].GeneratedAt) })
	history.UpdatedAt = time.Now().UTC()
	return history, nil
}

func looksLikeNativeHistory(history trendspkg.HistoryFile) bool {
	if strings.TrimSpace(history.Target) != "" || strings.TrimSpace(history.Version) != "" {
		return true
	}
	for _, run := range history.Runs {
		if !run.GeneratedAt.IsZero() {
			return true
		}
	}
	return false
}

func decodeArbitraryPayload(data []byte) (any, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err == nil {
		return payload, nil
	}
	if err := yaml.Unmarshal(data, &payload); err == nil {
		return payload, nil
	}
	return nil, fmt.Errorf("decode imported trend payload")
}

func importedRunsForSource(payload any, sourceType string) []trendspkg.HistoryRun {
	items := collectImportedItems(payload, sourceType)
	runs := make([]trendspkg.HistoryRun, 0, len(items))
	for _, item := range items {
		run, ok := importedHistoryRun(item)
		if ok {
			runs = append(runs, run)
		}
	}
	return runs
}

func collectImportedItems(payload any, sourceType string) []map[string]any {
	if historyMap, ok := payload.(map[string]any); ok {
		if embedded := nestedMap(historyMap, "cleanr", "history"); len(embedded) > 0 {
			if rows, ok := embedded["runs"].([]any); ok {
				return sliceOfMaps(rows)
			}
		}
		keys := importCollectionKeys(sourceType)
		for _, key := range keys {
			if rows, ok := historyMap[key].([]any); ok {
				return sliceOfMaps(rows)
			}
		}
		return []map[string]any{historyMap}
	}
	if rows, ok := payload.([]any); ok {
		return sliceOfMaps(rows)
	}
	return nil
}

func importCollectionKeys(sourceType string) []string {
	switch strings.TrimSpace(sourceType) {
	case "langsmith":
		return []string{"runs", "traces", "items"}
	case "openllmetry":
		return []string{"traces", "spans", "events", "logs", "items"}
	case "provider_logs":
		return []string{"logs", "events", "rows", "runs", "items"}
	default:
		return []string{"runs", "rows", "items", "logs", "events", "traces", "spans"}
	}
}

func importedHistoryRun(item map[string]any) (trendspkg.HistoryRun, bool) {
	if runMap := nestedMap(item, "cleanr", "history_run"); len(runMap) > 0 {
		if run, ok := decodeHistoryRun(runMap); ok {
			return run, true
		}
	}
	if reportMap := nestedMap(item, "cleanr", "report"); len(reportMap) > 0 {
		if report, ok := decodeReport(reportMap); ok {
			return trendspkg.BuildRun(report, buildID(report.Metadata)), true
		}
	}
	if reportMap := nestedMap(item, "report"); len(reportMap) > 0 {
		if report, ok := decodeReport(reportMap); ok {
			return trendspkg.BuildRun(report, buildID(report.Metadata)), true
		}
	}
	return genericImportedRun(item)
}

func decodeHistoryRun(value map[string]any) (trendspkg.HistoryRun, bool) {
	data, err := json.Marshal(value)
	if err != nil {
		return trendspkg.HistoryRun{}, false
	}
	var run trendspkg.HistoryRun
	if err := json.Unmarshal(data, &run); err != nil {
		return trendspkg.HistoryRun{}, false
	}
	if run.GeneratedAt.IsZero() {
		run.GeneratedAt = time.Now().UTC()
	}
	return run, true
}

func decodeReport(value map[string]any) (core.Report, bool) {
	data, err := json.Marshal(value)
	if err != nil {
		return core.Report{}, false
	}
	var report core.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return core.Report{}, false
	}
	if report.GeneratedAt.IsZero() {
		report.GeneratedAt = time.Now().UTC()
	}
	return report, true
}

func genericImportedRun(item map[string]any) (trendspkg.HistoryRun, bool) {
	generatedAt := firstTime(
		stringValue(item["generated_at"]),
		stringValue(item["start_time"]),
		stringValue(item["started_at"]),
		stringValue(item["timestamp"]),
		stringValue(item["created_at"]),
	)
	if generatedAt.IsZero() {
		return trendspkg.HistoryRun{}, false
	}
	duration := time.Duration(int64Value(item["duration_ms"])) * time.Millisecond
	if duration == 0 {
		startedAt := firstTime(stringValue(item["start_time"]), stringValue(item["started_at"]))
		endedAt := firstTime(stringValue(item["end_time"]), stringValue(item["ended_at"]))
		if !startedAt.IsZero() && !endedAt.IsZero() && endedAt.After(startedAt) {
			duration = endedAt.Sub(startedAt)
		}
	}
	passed := boolValue(item["passed"])
	if status := strings.ToLower(strings.TrimSpace(stringValue(item["status"]))); status != "" {
		switch status {
		case "success", "succeeded", "completed", "passed", "ok":
			passed = true
		case "error", "failed", "failure":
			passed = false
		}
	}
	buildID := firstNonEmpty(
		stringValue(item["build_id"]),
		stringValue(item["run_id"]),
		stringValue(item["trace_id"]),
		stringValue(item["id"]),
	)
	failedSuites := intValue(item["failed_suites"])
	failedCases := intValue(item["failed_cases"])
	reportMap := nestedMap(item, "cleanr")
	metadata := importedMetadata(reportMap, item)
	return trendspkg.HistoryRun{
		BuildID:      buildID,
		GeneratedAt:  generatedAt,
		Passed:       passed,
		Duration:     duration,
		FailedSuites: failedSuites,
		FailedCases:  failedCases,
		Metadata:     metadata,
	}, true
}

func importedMetadata(cleanrMeta, item map[string]any) *core.RunMetadata {
	model := firstNonEmpty(stringValue(cleanrMeta["provider_model"]), stringValue(item["model"]), stringValue(item["name"]))
	targetType := firstNonEmpty(stringValue(cleanrMeta["target_type"]), stringValue(item["provider"]), stringValue(item["source"]))
	if model == "" && targetType == "" {
		return nil
	}
	return &core.RunMetadata{
		ProviderModel: model,
		TargetType:    targetType,
	}
}

func importTargetName(source core.TrendSourceConfig) string {
	return firstNonEmpty(strings.TrimSpace(source.Project), strings.TrimSpace(source.Name), strings.TrimSpace(source.Type), "external-history")
}

func nestedMap(root map[string]any, path ...string) map[string]any {
	cur := any(root)
	for _, key := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = next[key]
	}
	out, _ := cur.(map[string]any)
	return out
}

func sliceOfMaps(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped, ok := item.(map[string]any); ok {
			out = append(out, mapped)
		}
	}
	return out
}

func firstTime(values ...string) time.Time {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, raw); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return false
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
