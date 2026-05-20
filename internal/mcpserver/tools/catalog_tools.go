package tools

import "context"

func callDescribeSuites(_ context.Context, _ map[string]any) (Result, error) {
	out := suiteCatalogOutput{
		Suites: []suiteDescriptor{
			{Name: "prompt_injection", Description: "Checks refusal and boundary handling for adversarial prompt overrides.", ConfigFields: []string{"enabled", "block_indicators"}},
			{Name: "security", Description: "Checks for secret leakage, PII-like output, and dangerous tool instructions.", ConfigFields: []string{"enabled", "leak_patterns", "max_pii_matches", "dangerous_tool_indicators", "secret_exposure_indicators"}},
			{Name: "load", Description: "Checks concurrency behavior, latency budgets, and error-rate thresholds.", ConfigFields: []string{"enabled", "virtual_users", "requests_per_user", "max_error_rate_pct", "p95_latency_ms"}},
			{Name: "chaos", Description: "Checks resilience under degraded conditions such as tight deadlines or noisy context.", ConfigFields: []string{"enabled", "faults", "timeout_scale", "noise_bytes", "max_error_rate_pct"}},
			{Name: "drift", Description: "Checks response stability across repeated executions.", ConfigFields: []string{"enabled", "iterations", "max_normalized_drift", "stable_tags", "min_consistency_score"}},
			{Name: "token_optimization", Description: "Checks prompt and response token budgets, ratios, and duplication.", ConfigFields: []string{"enabled", "max_input_tokens", "max_output_tokens", "max_total_tokens", "max_output_input_ratio", "max_prompt_duplication_ratio", "max_response_duplication_ratio", "suggested_max_output_tokens"}},
		},
	}
	return structuredToolResult(out, "described built-in cleanr suites"), nil
}

func callSupportedTargets(_ context.Context, _ map[string]any) (Result, error) {
	out := targetCatalogOutput{
		Targets: []targetDescriptor{
			{Type: "http", Description: "Generic HTTP target for chat, completion, agent, or tool-calling APIs.", ConfigFields: []string{"name", "url", "method", "headers", "timeout_ms", "prompt_field", "system_field", "response_field", "request_template"}},
			{Type: "openai", Description: "Native OpenAI target using the Responses or Chat Completions API.", ConfigFields: []string{"name", "timeout_ms", "headers", "openai.api_mode", "openai.model", "openai.api_key_env", "openai.base_url", "openai.organization", "openai.project"}},
			{Type: "anthropic", Description: "Native Anthropic target using the Messages API.", ConfigFields: []string{"name", "timeout_ms", "headers", "anthropic.model", "anthropic.api_key_env", "anthropic.base_url", "anthropic.version", "anthropic.max_tokens"}},
		},
	}
	return structuredToolResult(out, "described supported cleanr targets"), nil
}
