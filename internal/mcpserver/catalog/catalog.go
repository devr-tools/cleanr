package catalog

import (
	"context"

	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
)

func SuiteDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_describe_suites",
		Title:       "Describe cleanr suites",
		Description: "Return the built-in cleanr suites, what they check, and their key config fields.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"suites": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
			},
			"required": []string{"suites"},
		},
	}
}

func TargetDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_supported_targets",
		Title:       "Describe cleanr targets",
		Description: "Return the supported cleanr target types and their key config fields.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"targets": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
			},
			"required": []string{"targets"},
		},
	}
}

func DescribeSuites(_ context.Context, _ map[string]any) (toolkit.Result, error) {
	out := toolkit.SuiteCatalogOutput{
		Suites: []toolkit.SuiteDescriptor{
			{Name: "prompt_injection", Description: "Checks refusal and boundary handling for adversarial prompt overrides.", ConfigFields: []string{"enabled", "block_indicators"}},
			{Name: "security", Description: "Checks for secret leakage, PII-like output, and dangerous tool instructions.", ConfigFields: []string{"enabled", "leak_patterns", "max_pii_matches", "dangerous_tool_indicators", "secret_exposure_indicators"}},
			{Name: "load", Description: "Checks concurrency behavior, latency budgets, and error-rate thresholds.", ConfigFields: []string{"enabled", "virtual_users", "requests_per_user", "max_error_rate_pct", "p95_latency_ms"}},
			{Name: "chaos", Description: "Checks resilience under degraded conditions such as tight deadlines or noisy context.", ConfigFields: []string{"enabled", "faults", "timeout_scale", "noise_bytes", "max_error_rate_pct"}},
			{Name: "drift", Description: "Checks response stability across repeated executions.", ConfigFields: []string{"enabled", "iterations", "max_normalized_drift", "stable_tags", "min_consistency_score"}},
			{Name: "shadow_state", Description: "Checks observed file mutations against approved write paths.", ConfigFields: []string{"enabled", "roots", "allowed_write_paths"}},
			{Name: "provenance", Description: "Checks whether untrusted context can override policy or trigger privileged actions.", ConfigFields: []string{"enabled", "block_indicators", "validation_indicators", "sensitive_indicators", "privileged_tool_names"}},
			{Name: "claim_trace", Description: "Checks whether claimed citations, tool use, approvals, and state changes are backed by trace evidence.", ConfigFields: []string{"enabled", "citation_indicators", "tool_claim_indicators", "approval_indicators", "state_change_indicators"}},
			{Name: "memory_safety", Description: "Checks whether stale, revoked, poisoned, or cross-user memory flows into responses or tool calls.", ConfigFields: []string{"enabled"}},
			{Name: "token_optimization", Description: "Checks prompt and response token budgets, ratios, and duplication.", ConfigFields: []string{"enabled", "max_input_tokens", "max_output_tokens", "max_total_tokens", "max_output_input_ratio", "max_prompt_duplication_ratio", "max_response_duplication_ratio", "suggested_max_output_tokens"}},
			{Name: "llm_judge", Description: "Grades each response with a separate judge model: rubric/reference scoring with self-consistency sampling, or pairwise comparison against a baseline with position-swap debiasing.", ConfigFields: []string{"enabled", "mode", "provider", "baseline", "criteria", "scale", "min_score", "min_win_rate", "samples", "max_disagreement", "require_reference", "stable_tags"}},
		},
	}
	return toolkit.StructuredToolResult(out, "described built-in cleanr suites"), nil
}

func SupportedTargets(_ context.Context, _ map[string]any) (toolkit.Result, error) {
	out := toolkit.TargetCatalogOutput{
		Targets: []toolkit.TargetDescriptor{
			{Type: "http", Description: "Generic HTTP target for chat, completion, agent, or tool-calling APIs.", ConfigFields: []string{"name", "url", "method", "headers", "timeout_ms", "prompt_field", "system_field", "response_field", "request_template"}},
			{Type: "openai", Description: "Native OpenAI target using the Responses or Chat Completions API.", ConfigFields: []string{"name", "timeout_ms", "headers", "openai.api_mode", "openai.model", "openai.api_key_env", "openai.base_url", "openai.organization", "openai.project"}},
			{Type: "anthropic", Description: "Native Anthropic target using the Messages API.", ConfigFields: []string{"name", "timeout_ms", "headers", "anthropic.model", "anthropic.api_key_env", "anthropic.base_url", "anthropic.version", "anthropic.max_tokens"}},
		},
	}
	return toolkit.StructuredToolResult(out, "described supported cleanr targets"), nil
}
