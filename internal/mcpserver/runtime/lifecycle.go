package runtime

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
)

var GenerateScenarioDatasetFunc = cleanr.GenerateScenarioDataset

func GenerateDatasetDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_generate_dataset",
		Title:       "Generate cleanr scenario dataset",
		Description: "Generate a scenario dataset from a cleanr config with scenario generation enabled.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config": map[string]any{
					"type":        "string",
					"description": "Raw cleanr config content.",
				},
				"config_path": map[string]any{
					"type":        "string",
					"description": "Local path to a cleanr config file.",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Config format when config is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"output_format": map[string]any{
					"type":        "string",
					"description": "Rendered dataset format.",
					"enum":        []string{"json", "yaml"},
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format":       map[string]any{"type": "string"},
				"dataset_text": map[string]any{"type": "string"},
				"dataset":      map[string]any{"type": "object"},
			},
			"required": []string{"format", "dataset_text", "dataset"},
		},
	}
}

func ReviewDatasetDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_review_dataset",
		Title:       "Review cleanr scenario dataset",
		Description: "Review a generated scenario dataset against a cleanr config and return reviewed plus approved artifacts.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config": map[string]any{
					"type":        "string",
					"description": "Raw cleanr config content.",
				},
				"config_path": map[string]any{
					"type":        "string",
					"description": "Local path to a cleanr config file.",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Config format when config is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"dataset": map[string]any{
					"type":        "string",
					"description": "Raw scenario dataset content.",
				},
				"dataset_path": map[string]any{
					"type":        "string",
					"description": "Local path to a scenario dataset file.",
				},
				"dataset_format": map[string]any{
					"type":        "string",
					"description": "Dataset format when dataset is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"policy": map[string]any{
					"type":        "string",
					"description": "Optional inline dataset review policy.",
				},
				"policy_path": map[string]any{
					"type":        "string",
					"description": "Optional path to a dataset review policy file.",
				},
				"policy_format": map[string]any{
					"type":        "string",
					"description": "Policy format when policy is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"output_format": map[string]any{
					"type":        "string",
					"description": "Rendered reviewed dataset format.",
					"enum":        []string{"json", "yaml"},
				},
				"approve":            stringArraySchema("Scenario names to approve."),
				"reject":             stringArraySchema("Scenario names to reject."),
				"promote_stable":     stringArraySchema("Scenario names to tag stable."),
				"promote_regression": stringArraySchema("Scenario names to tag regression."),
				"add_tags":           mapOfStringArraysSchema("Per-scenario tags to add."),
				"set_tags":           mapOfStringArraysSchema("Per-scenario tags to replace."),
				"set_metadata":       nestedStringMapSchema("Per-scenario metadata to set."),
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format":                map[string]any{"type": "string"},
				"reviewed_dataset_text": map[string]any{"type": "string"},
				"approved_dataset_text": map[string]any{"type": "string"},
				"reviewed_dataset":      map[string]any{"type": "object"},
				"approved_dataset":      map[string]any{"type": "object"},
			},
			"required": []string{"format", "reviewed_dataset_text", "approved_dataset_text", "reviewed_dataset", "approved_dataset"},
		},
	}
}

func AnalyzeTrendsDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_analyze_trends",
		Title:       "Analyze cleanr trend history",
		Description: "Analyze retained trend history and render a trend summary.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"history": map[string]any{
					"type":        "string",
					"description": "Raw trend history content.",
				},
				"history_path": map[string]any{
					"type":        "string",
					"description": "Local path to a trend history file.",
				},
				"history_format": map[string]any{
					"type":        "string",
					"description": "Trend history format when history is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"window": map[string]any{
					"type":        "integer",
					"description": "Optional retained-run window. Use 0 for the full retained history.",
					"minimum":     0,
				},
				"output_format": map[string]any{
					"type":        "string",
					"description": "Rendered trend analysis format.",
					"enum":        []string{"text", "json"},
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format":   map[string]any{"type": "string"},
				"rendered": map[string]any{"type": "string"},
				"analysis": map[string]any{"type": "object"},
			},
			"required": []string{"format", "rendered", "analysis"},
		},
	}
}

func ExplainFailuresDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_explain_failures",
		Title:       "Explain cleanr replay failures",
		Description: "Summarize replay artifact failures into grouped buckets and per-case explanations.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"replay": map[string]any{
					"type":        "string",
					"description": "Raw replay artifact content.",
				},
				"replay_path": map[string]any{
					"type":        "string",
					"description": "Local path to a replay artifact file.",
				},
				"replay_format": map[string]any{
					"type":        "string",
					"description": "Replay artifact format when replay is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"max_cases": map[string]any{
					"type":        "integer",
					"description": "Optional maximum number of per-case explanations to return.",
					"minimum":     0,
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target":        map[string]any{"type": "string"},
				"build_id":      map[string]any{"type": "string"},
				"failure_count": map[string]any{"type": "integer"},
				"bucket_count":  map[string]any{"type": "integer"},
				"buckets": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
				"explanations": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
				"summary": map[string]any{"type": "string"},
			},
			"required": []string{"failure_count", "bucket_count", "summary"},
		},
	}
}

func GenerateDataset(ctx context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.GenerateDatasetArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	cfg, err := toolkit.LoadConfigSource(toolkit.ConfigSource{
		Config:     input.Config,
		ConfigPath: input.ConfigPath,
		Format:     input.Format,
	})
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := toolkit.GuardMCPConfig(cfg); err != nil {
		return toolkit.Result{}, err
	}

	// A real client is required: the generation path calls client.Do directly,
	// so a nil *http.Client panics on first use. Mirror the CLI generate
	// command's provider-timeout client.
	client := &http.Client{Timeout: cfg.ScenarioGeneration.Provider.Timeout()}
	dataset, err := GenerateScenarioDatasetFunc(ctx, cfg, client)
	if err != nil {
		return toolkit.Result{}, err
	}

	format := toolkit.NormalizeDataFormat(input.OutputFormat)
	rendered, err := toolkit.EncodeData(dataset, format)
	if err != nil {
		return toolkit.Result{}, err
	}

	out := toolkit.GenerateDatasetOutput{
		Format:      format,
		DatasetText: rendered,
		Dataset:     dataset,
	}
	return toolkit.StructuredToolResult(out, rendered), nil
}

func ReviewDataset(_ context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.ReviewDatasetArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	cfg, err := toolkit.LoadConfigSource(toolkit.ConfigSource{
		Config:     input.Config,
		ConfigPath: input.ConfigPath,
		Format:     input.Format,
	})
	if err != nil {
		return toolkit.Result{}, err
	}

	dataset, err := toolkit.LoadScenarioDatasetSource(input.DatasetPath, input.Dataset, input.DatasetFormat)
	if err != nil {
		return toolkit.Result{}, err
	}
	policy, err := toolkit.LoadDatasetReviewPolicySource(input.PolicyPath, input.Policy, input.PolicyFormat)
	if err != nil {
		return toolkit.Result{}, err
	}

	reviewed, err := cleanr.ReviewDatasetAgainstConfig(cfg, dataset, cleanr.DatasetReviewOptions{
		Approve:           input.Approve,
		Reject:            input.Reject,
		PromoteStable:     input.PromoteStable,
		PromoteRegression: input.PromoteRegression,
		AddTags:           input.AddTags,
		SetTags:           input.SetTags,
		SetMetadata:       input.SetMetadata,
		Policy:            policy,
	})
	if err != nil {
		return toolkit.Result{}, err
	}
	reviewed = cleanr.FinalizeReviewedScenarioDataset(reviewed)
	approved := cleanr.ApprovedDatasetFromReview(reviewed)

	format := toolkit.NormalizeDataFormat(input.OutputFormat)
	reviewedText, err := toolkit.EncodeData(reviewed, format)
	if err != nil {
		return toolkit.Result{}, err
	}
	approvedText, err := toolkit.EncodeData(approved, format)
	if err != nil {
		return toolkit.Result{}, err
	}

	out := toolkit.ReviewDatasetOutput{
		Format:              format,
		ReviewedDatasetText: reviewedText,
		ApprovedDatasetText: approvedText,
		ReviewedDataset:     reviewed,
		ApprovedDataset:     approved,
	}
	return toolkit.StructuredToolResult(out, reviewedText), nil
}

func AnalyzeTrends(_ context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.AnalyzeTrendsArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	analysis, err := toolkit.AnalyzeTrendHistorySource(input.HistoryPath, input.History, input.HistoryFormat, input.Window)
	if err != nil {
		return toolkit.Result{}, err
	}
	format := toolkit.NormalizeTrendFormat(input.OutputFormat)
	rendered, err := toolkit.RenderTrendAnalysis(analysis, format)
	if err != nil {
		return toolkit.Result{}, err
	}

	out := toolkit.AnalyzeTrendsOutput{
		Format:   format,
		Rendered: rendered,
		Analysis: analysis,
	}
	return toolkit.StructuredToolResult(out, rendered), nil
}

func ExplainFailures(_ context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.ExplainFailuresArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	artifact, err := toolkit.LoadReplayArtifactSource(input.ReplayPath, input.Replay, input.ReplayFormat)
	if err != nil {
		return toolkit.Result{}, err
	}

	explanations, buckets := explainReplayArtifactFailures(artifact, input.MaxCases)
	summary := renderFailureSummary(artifact, explanations, buckets)
	out := toolkit.ExplainFailuresOutput{
		Target:       artifact.Target,
		BuildID:      artifact.BuildID,
		FailureCount: len(artifact.Failures),
		BucketCount:  len(buckets),
		Buckets:      buckets,
		Explanations: explanations,
		Summary:      summary,
	}
	return toolkit.StructuredToolResult(out, summary), nil
}

func explainReplayArtifactFailures(artifact cleanr.ReplayArtifact, maxCases int) ([]toolkit.FailureExplanation, []cleanr.FailureBucket) {
	if maxCases < 0 {
		maxCases = 0
	}

	bucketMap := map[string]*cleanr.FailureBucket{}
	explanations := make([]toolkit.FailureExplanation, 0, len(artifact.Failures))
	for _, failure := range artifact.Failures {
		explanation := toolkit.FailureExplanation{
			Suite:         failure.Suite,
			Case:          failure.Name,
			Failed:        failure.Failed,
			PrimaryReason: primaryFailureReason(failure),
			Findings:      findingLines(failure.Findings),
		}
		if failure.Scenario != nil {
			explanation.ScenarioName = failure.Scenario.Name
		}
		explanation.EvidenceHighlights = evidenceHighlights(failure.Evidence)
		explanations = append(explanations, explanation)

		for _, signature := range failureSignatures(failure) {
			bucket := bucketMap[signature]
			if bucket == nil {
				bucket = &cleanr.FailureBucket{Signature: signature}
				bucketMap[signature] = bucket
			}
			bucket.Count++
			bucket.Cases = append(bucket.Cases, failure.Suite+"/"+failure.Name)
		}
	}

	sort.Slice(explanations, func(i, j int) bool {
		if explanations[i].Suite == explanations[j].Suite {
			return explanations[i].Case < explanations[j].Case
		}
		return explanations[i].Suite < explanations[j].Suite
	})
	if maxCases > 0 && len(explanations) > maxCases {
		explanations = explanations[:maxCases]
	}

	buckets := make([]cleanr.FailureBucket, 0, len(bucketMap))
	for _, bucket := range bucketMap {
		sort.Strings(bucket.Cases)
		bucket.Cases = compactStrings(bucket.Cases)
		buckets = append(buckets, *bucket)
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count == buckets[j].Count {
			return buckets[i].Signature < buckets[j].Signature
		}
		return buckets[i].Count > buckets[j].Count
	})
	return explanations, buckets
}

func renderFailureSummary(artifact cleanr.ReplayArtifact, explanations []toolkit.FailureExplanation, buckets []cleanr.FailureBucket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Explained %d failures", len(artifact.Failures))
	if strings.TrimSpace(artifact.Target) != "" {
		fmt.Fprintf(&b, " for %s", artifact.Target)
	}
	if strings.TrimSpace(artifact.BuildID) != "" {
		fmt.Fprintf(&b, " (build %s)", artifact.BuildID)
	}
	b.WriteString(".")
	if len(buckets) > 0 {
		b.WriteString("\n\nTop failure buckets:")
		limit := 3
		if len(buckets) < limit {
			limit = len(buckets)
		}
		for _, bucket := range buckets[:limit] {
			fmt.Fprintf(&b, "\n- %s (%d)", bucket.Signature, bucket.Count)
		}
	}
	if len(explanations) > 0 {
		b.WriteString("\n\nCase explanations:")
		for _, explanation := range explanations {
			fmt.Fprintf(&b, "\n- %s/%s: %s", explanation.Suite, explanation.Case, explanation.PrimaryReason)
		}
	}
	return b.String()
}

func primaryFailureReason(failure cleanr.ReplayArtifactCase) string {
	if len(failure.Findings) > 0 {
		finding := failure.Findings[0]
		if severity := strings.TrimSpace(finding.Severity); severity != "" {
			return strings.ToLower(severity) + ": " + strings.TrimSpace(finding.Message)
		}
		if message := strings.TrimSpace(finding.Message); message != "" {
			return message
		}
	}
	highlights := evidenceHighlights(failure.Evidence)
	if len(highlights) > 0 {
		return highlights[0]
	}
	if failure.Failed {
		return "case failed with no retained finding details"
	}
	return "case emitted findings without being marked failed"
}

func findingLines(findings []cleanr.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		message := strings.TrimSpace(finding.Message)
		if message == "" {
			continue
		}
		if severity := strings.TrimSpace(finding.Severity); severity != "" {
			out = append(out, strings.ToLower(severity)+": "+message)
			continue
		}
		out = append(out, message)
	}
	return out
}

func evidenceHighlights(evidence map[string]any) []string {
	if len(evidence) == 0 {
		return nil
	}
	keys := make([]string, 0, len(evidence))
	for key := range evidence {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(evidence[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		value = strings.ReplaceAll(value, "\n", " ")
		if len(value) > 120 {
			value = value[:117] + "..."
		}
		out = append(out, fmt.Sprintf("%s=%s", key, value))
		if len(out) == 3 {
			break
		}
	}
	return out
}

func failureSignatures(failure cleanr.ReplayArtifactCase) []string {
	out := make([]string, 0, len(failure.Findings))
	for _, finding := range failure.Findings {
		message := normalizeFailureSignature(finding.Message)
		if message != "" {
			out = append(out, message)
		}
	}
	if len(out) == 0 {
		out = append(out, primaryFailureReason(failure))
	}
	return compactStrings(out)
}

func normalizeFailureSignature(message string) string {
	message = strings.ToLower(strings.TrimSpace(message))
	message = strings.Join(strings.Fields(message), " ")
	return message
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func stringArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
	}
}

func mapOfStringArraysSchema(description string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": description,
		"additionalProperties": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	}
}

func nestedStringMapSchema(description string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": description,
		"additionalProperties": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
		},
	}
}
