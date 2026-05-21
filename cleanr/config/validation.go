package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"cleanr/cleanr/core"
)

func ValidateConfig(cfg core.Config) error {
	var errs ValidationErrors

	switch cfg.Target.TargetType() {
	case "http":
		requireNonEmpty(&errs, "target.url", cfg.Target.URL, "set target.url to the full API endpoint URL")
		requireNonEmpty(&errs, "target.prompt_field", cfg.Target.PromptField, "set target.prompt_field to the request field that receives the prompt text")
		requireNonEmpty(&errs, "target.response_field", cfg.Target.ResponseField, "set target.response_field to the JSON path that contains the model text response")
		if rawURL := strings.TrimSpace(cfg.Target.URL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add("target.url", "must be an absolute http(s) URL", "use a value such as http://localhost:8080/v1/chat or https://api.example.com/v1/chat")
			}
		}
	case "openai":
		requireNonEmpty(&errs, "target.openai.model", cfg.Target.OpenAI.Model, "set the OpenAI model name, for example gpt-4o-mini or gpt-4.1-mini")
		switch cfg.Target.OpenAI.APIModeValue() {
		case "responses", "chat_completions":
		default:
			errs.Add("target.openai.api_mode", "must be one of responses or chat_completions", "use responses for new projects or chat_completions for legacy-compatible message requests")
		}
		if rawURL := strings.TrimSpace(cfg.Target.OpenAI.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add("target.openai.base_url", "must be an absolute http(s) URL", "use a value such as https://api.openai.com/v1 or a compatible base URL for testing")
			}
		}
	case "anthropic":
		requireNonEmpty(&errs, "target.anthropic.model", cfg.Target.Anthropic.Model, "set the Anthropic model name, for example claude-sonnet-4-20250514")
		if rawURL := strings.TrimSpace(cfg.Target.Anthropic.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add("target.anthropic.base_url", "must be an absolute http(s) URL", "use a value such as https://api.anthropic.com/v1 or a compatible base URL for testing")
			}
		}
		if cfg.Target.Anthropic.MaxTokens < 0 {
			errs.Add("target.anthropic.max_tokens", "must be >= 0", "set a positive max_tokens budget or omit the field to use the default")
		}
	default:
		errs.Add("target.type", "must be one of http, openai, or anthropic", "omit target.type for the default HTTP adapter, or set it to openai or anthropic for a native provider adapter")
	}

	if cfg.Target.TimeoutMS < 0 {
		errs.Add("target.timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
	}

	scenarioNames := make(map[string]int, len(cfg.Scenarios))
	if len(cfg.Scenarios) == 0 {
		errs.Add("scenarios", "at least one scenario is required", "add a scenario with both name and input so cleanr has something to execute")
	}
	for i, scenario := range cfg.Scenarios {
		prefix := fmt.Sprintf("scenarios[%d]", i)
		requireNonEmpty(&errs, prefix+".name", scenario.Name, "set a short stable scenario name, for example \"happy-path\"")
		requireNonEmpty(&errs, prefix+".input", scenario.Input, "set the end-user prompt or test input for this scenario")
		if name := strings.TrimSpace(scenario.Name); name != "" {
			if first, ok := scenarioNames[name]; ok {
				errs.Add(prefix+".name", fmt.Sprintf("duplicates scenarios[%d].name", first), "rename duplicate scenarios so each scenario name is unique in reports")
			} else {
				scenarioNames[name] = i
			}
		}
		for j, source := range scenario.ContextSources {
			validateContextSource(&errs, fmt.Sprintf("%s.context_sources[%d]", prefix, j), source)
		}
		if len(scenario.MemoryReplay) == 1 {
			errs.Add(prefix+".memory_replay", "must contain at least two sessions", "add a second ordered session so memory replay can be evaluated across a session boundary")
		}
		replaySessionIDs := make(map[string]int, len(scenario.MemoryReplay))
		for j, session := range scenario.MemoryReplay {
			sessionPrefix := fmt.Sprintf("%s.memory_replay[%d]", prefix, j)
			validateMemoryReplaySession(&errs, sessionPrefix, session)
			if sessionID := strings.TrimSpace(session.SessionID); sessionID != "" {
				if first, ok := replaySessionIDs[sessionID]; ok {
					errs.Add(sessionPrefix+".session_id", fmt.Sprintf("duplicates %s.memory_replay[%d].session_id", prefix, first), "set a unique traced session_id for each replay step")
				} else {
					replaySessionIDs[sessionID] = j
				}
			}
		}
		for j, mutation := range scenario.ExpectedMutations {
			validateExpectedMutation(&errs, fmt.Sprintf("%s.expected_mutations[%d]", prefix, j), mutation)
		}
		for j, change := range scenario.ExpectedStateChanges {
			validateExpectedStateChange(&errs, fmt.Sprintf("%s.expected_state_changes[%d]", prefix, j), change)
		}
		for j, assertion := range scenario.Assertions {
			validateAssertion(&errs, fmt.Sprintf("%s.assertions[%d]", prefix, j), assertion)
		}
	}

	if cfg.Suites.Load.Enabled {
		requirePositiveInt(&errs, "suites.load.virtual_users", cfg.Suites.Load.VirtualUsers, "set virtual_users to at least 1 when the load suite is enabled")
		requirePositiveInt(&errs, "suites.load.requests_per_user", cfg.Suites.Load.RequestsPerUser, "set requests_per_user to at least 1 when the load suite is enabled")
		if cfg.Suites.Load.MaxErrorRatePct < 0 || cfg.Suites.Load.MaxErrorRatePct > 100 {
			errs.Add("suites.load.max_error_rate_pct", "must be between 0 and 100", "use a whole-number percentage such as 5 or 25")
		}
		if cfg.Suites.Load.P95LatencyMS < 0 {
			errs.Add("suites.load.p95_latency_ms", "must be >= 0", "set a positive latency budget in milliseconds")
		}
	}

	for i, pattern := range cfg.Suites.Security.LeakPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			errs.Add(fmt.Sprintf("suites.security.leak_patterns[%d]", i), "must be a valid Go regular expression", "fix the pattern syntax or remove the entry if it is no longer needed")
		}
	}

	if cfg.Suites.Chaos.Enabled {
		allowedFaults := map[string]struct{}{
			"tight_deadline":   {},
			"context_overflow": {},
			"duplicate_turn":   {},
		}
		for i, fault := range cfg.Suites.Chaos.Faults {
			if _, ok := allowedFaults[fault]; !ok {
				errs.Add(fmt.Sprintf("suites.chaos.faults[%d]", i), "must be one of tight_deadline, context_overflow, or duplicate_turn", "replace the value with a supported built-in chaos fault")
			}
		}
		if cfg.Suites.Chaos.TimeoutScale <= 0 {
			errs.Add("suites.chaos.timeout_scale", "must be > 0", "use a fractional multiplier such as 0.4 to shorten the timeout")
		}
		if cfg.Suites.Chaos.NoiseBytes < 0 {
			errs.Add("suites.chaos.noise_bytes", "must be >= 0", "set a non-negative number of injected bytes")
		}
		if cfg.Suites.Chaos.MaxErrorRate < 0 || cfg.Suites.Chaos.MaxErrorRate > 100 {
			errs.Add("suites.chaos.max_error_rate_pct", "must be between 0 and 100", "use a whole-number percentage such as 35")
		}
	}

	if cfg.Suites.Drift.Enabled {
		if cfg.Suites.Drift.Iterations < 2 {
			errs.Add("suites.drift.iterations", "must be >= 2", "set iterations to 2 or more so drift can compare repeated runs")
		}
		if cfg.Suites.Drift.MaxNormalizedDrift < 0 || cfg.Suites.Drift.MaxNormalizedDrift > 1 {
			errs.Add("suites.drift.max_normalized_drift", "must be between 0 and 1", "use a decimal threshold such as 0.3")
		}
		if cfg.Suites.Drift.MaxSemanticDrift < 0 || cfg.Suites.Drift.MaxSemanticDrift > 1 {
			errs.Add("suites.drift.max_semantic_drift", "must be between 0 and 1", "use a decimal threshold such as 0.25")
		}
		if cfg.Suites.Drift.MaxSnapshotDrift < 0 || cfg.Suites.Drift.MaxSnapshotDrift > 1 {
			errs.Add("suites.drift.max_snapshot_drift", "must be between 0 and 1", "use a decimal threshold such as 0.18")
		}
		if cfg.Suites.Drift.MaxSemanticSnapshotDrift < 0 || cfg.Suites.Drift.MaxSemanticSnapshotDrift > 1 {
			errs.Add("suites.drift.max_semantic_snapshot_drift", "must be between 0 and 1", "use a decimal threshold such as 0.2")
		}
		if cfg.Suites.Drift.MinConsistencyScore < 0 || cfg.Suites.Drift.MinConsistencyScore > 1 {
			errs.Add("suites.drift.min_consistency_score", "must be between 0 and 1", "use a decimal threshold such as 0.7")
		}
		if cfg.Suites.Drift.MinSemanticConsistencyScore < 0 || cfg.Suites.Drift.MinSemanticConsistencyScore > 1 {
			errs.Add("suites.drift.min_semantic_consistency_score", "must be between 0 and 1", "use a decimal threshold such as 0.75")
		}
	}

	if cfg.Suites.ShadowState.Enabled {
		if len(cfg.Suites.ShadowState.Roots) == 0 {
			errs.Add("suites.shadow_state.roots", "must contain at least one path", "set one or more files or directories that cleanr should snapshot before and after each run")
		}
		for i, root := range cfg.Suites.ShadowState.Roots {
			requireNonEmpty(&errs, fmt.Sprintf("suites.shadow_state.roots[%d]", i), root, "set a file or directory path to observe for side effects")
		}
		for i, path := range cfg.Suites.ShadowState.AllowedWritePaths {
			requireNonEmpty(&errs, fmt.Sprintf("suites.shadow_state.allowed_write_paths[%d]", i), path, "set a file or directory path where mutations are allowed")
		}
	}

	if cfg.Suites.Provenance.Enabled {
		for i, indicator := range cfg.Suites.Provenance.BlockIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.provenance.block_indicators[%d]", i), indicator, "set a non-empty refusal marker")
		}
		for i, indicator := range cfg.Suites.Provenance.ValidationIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.provenance.validation_indicators[%d]", i), indicator, "set a non-empty validation marker")
		}
		for i, indicator := range cfg.Suites.Provenance.SensitiveIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.provenance.sensitive_indicators[%d]", i), indicator, "set a non-empty sensitive-data marker")
		}
		for i, name := range cfg.Suites.Provenance.PrivilegedToolNames {
			requireNonEmpty(&errs, fmt.Sprintf("suites.provenance.privileged_tool_names[%d]", i), name, "set a non-empty tool name")
		}
		for i, name := range cfg.Suites.Provenance.ApprovalRequiredToolNames {
			requireNonEmpty(&errs, fmt.Sprintf("suites.provenance.approval_required_tool_names[%d]", i), name, "set a non-empty tool name")
		}
		for i, name := range cfg.Suites.Provenance.ApprovedSinkToolNames {
			requireNonEmpty(&errs, fmt.Sprintf("suites.provenance.approved_sink_tool_names[%d]", i), name, "set a non-empty tool name")
		}
	}

	if cfg.Suites.ClaimTrace.Enabled {
		for i, indicator := range cfg.Suites.ClaimTrace.CitationIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.claim_trace.citation_indicators[%d]", i), indicator, "set a non-empty citation marker")
		}
		for i, indicator := range cfg.Suites.ClaimTrace.ToolClaimIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.claim_trace.tool_claim_indicators[%d]", i), indicator, "set a non-empty tool-claim marker")
		}
		for i, indicator := range cfg.Suites.ClaimTrace.ApprovalIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.claim_trace.approval_indicators[%d]", i), indicator, "set a non-empty approval marker")
		}
		for i, indicator := range cfg.Suites.ClaimTrace.StateChangeIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.claim_trace.state_change_indicators[%d]", i), indicator, "set a non-empty state-change marker")
		}
	}

	if cfg.Suites.ReleasePolicy.Enabled {
		for i, indicator := range cfg.Suites.ReleasePolicy.SensitiveIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.release_policy.sensitive_indicators[%d]", i), indicator, "set a non-empty sensitive-data marker")
		}
		for i, indicator := range cfg.Suites.ReleasePolicy.ReadOnlyIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.release_policy.read_only_indicators[%d]", i), indicator, "set a non-empty read-only marker")
		}
		for i, indicator := range cfg.Suites.ReleasePolicy.MutatingIndicators {
			requireNonEmpty(&errs, fmt.Sprintf("suites.release_policy.mutating_indicators[%d]", i), indicator, "set a non-empty mutating marker")
		}
		for i, rule := range cfg.Suites.ReleasePolicy.Rules {
			validatePolicyRule(&errs, fmt.Sprintf("suites.release_policy.rules[%d]", i), rule)
		}
	}

	if cfg.Suites.TokenOptimization.Enabled {
		if cfg.Suites.TokenOptimization.MaxInputTokens < 0 {
			errs.Add("suites.token_optimization.max_input_tokens", "must be >= 0", "set a non-negative token budget or omit the field to use the default")
		}
		if cfg.Suites.TokenOptimization.MaxOutputTokens < 0 {
			errs.Add("suites.token_optimization.max_output_tokens", "must be >= 0", "set a non-negative token budget or omit the field to use the default")
		}
		if cfg.Suites.TokenOptimization.MaxTotalTokens < 0 {
			errs.Add("suites.token_optimization.max_total_tokens", "must be >= 0", "set a non-negative token budget or omit the field to use the default")
		}
		if cfg.Suites.TokenOptimization.MaxOutputInputRatio <= 0 {
			errs.Add("suites.token_optimization.max_output_input_ratio", "must be > 0", "use a positive ratio such as 1.4")
		}
		if cfg.Suites.TokenOptimization.MaxPromptDuplicationRatio < 0 || cfg.Suites.TokenOptimization.MaxPromptDuplicationRatio > 1 {
			errs.Add("suites.token_optimization.max_prompt_duplication_ratio", "must be between 0 and 1", "use a decimal ratio such as 0.18")
		}
		if cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio < 0 || cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio > 1 {
			errs.Add("suites.token_optimization.max_response_duplication_ratio", "must be between 0 and 1", "use a decimal ratio such as 0.12")
		}
		if cfg.Suites.TokenOptimization.SuggestedMaxOutputTokens < 0 {
			errs.Add("suites.token_optimization.suggested_max_output_tokens", "must be >= 0", "set a non-negative suggestion or omit the field to use the default")
		}
	}

	if format := strings.TrimSpace(cfg.Reporting.Format); format != "" {
		switch format {
		case "text", "json", "junit", "sarif":
		default:
			errs.Add("reporting.format", "must be one of text, json, junit, or sarif", "use one of the built-in report formats or omit the field for text output")
		}
	}
	if cfg.Reporting.TrendLimit < 0 {
		errs.Add("reporting.trend_limit", "must be >= 0", "use 0 to disable history trimming or set a positive run-retention count such as 30")
	}
	if !isValidTrendGatePreset(cfg.Reporting.TrendGates.Preset) {
		errs.Add("reporting.trend_gates.preset", "must be one of strict, moderate, or exploratory", "choose a built-in trend gate preset or remove the field to set thresholds manually")
	}
	if cfg.Reporting.TrendGates.Enabled {
		if strings.TrimSpace(cfg.Reporting.TrendFile) == "" {
			errs.Add("reporting.trend_file", "is required when trend gates are enabled", "set reporting.trend_file so cleanr can compare the current run to prior retained runs")
		}
		if cfg.Reporting.TrendGates.RequiredWindow < 2 {
			errs.Add("reporting.trend_gates.required_window", "must be >= 2", "use at least 2 retained runs so cleanr can compare the current build against a previous one")
		}
		if cfg.Reporting.TrendGates.MaxFailedSuitesDelta != nil && *cfg.Reporting.TrendGates.MaxFailedSuitesDelta < 0 {
			errs.Add("reporting.trend_gates.max_failed_suites_delta", "must be >= 0", "use a non-negative number of additional failed suites allowed between builds")
		}
		if cfg.Reporting.TrendGates.MaxFailedCasesDelta != nil && *cfg.Reporting.TrendGates.MaxFailedCasesDelta < 0 {
			errs.Add("reporting.trend_gates.max_failed_cases_delta", "must be >= 0", "use a non-negative number of additional failed cases allowed between builds")
		}
		if cfg.Reporting.TrendGates.MaxDurationIncreasePct != nil && *cfg.Reporting.TrendGates.MaxDurationIncreasePct < 0 {
			errs.Add("reporting.trend_gates.max_duration_increase_pct", "must be >= 0", "use a non-negative percentage such as 20 for a 20 percent duration increase budget")
		}
		if cfg.Reporting.TrendGates.MaxSemanticDriftDelta != nil && (*cfg.Reporting.TrendGates.MaxSemanticDriftDelta < 0 || *cfg.Reporting.TrendGates.MaxSemanticDriftDelta > 1) {
			errs.Add("reporting.trend_gates.max_semantic_drift_delta", "must be between 0 and 1", "use a decimal drift delta such as 0.08")
		}
		if cfg.Reporting.TrendGates.MaxBaselineSemanticDriftDelta != nil && (*cfg.Reporting.TrendGates.MaxBaselineSemanticDriftDelta < 0 || *cfg.Reporting.TrendGates.MaxBaselineSemanticDriftDelta > 1) {
			errs.Add("reporting.trend_gates.max_baseline_semantic_drift_delta", "must be between 0 and 1", "use a decimal drift delta such as 0.05")
		}
	}
	if cfg.Governance.Attestation.Enabled {
		requireNonEmpty(&errs, "governance.attestation.output", cfg.Governance.Attestation.Output, "set the attestation output path so cleanr can write the signed release-gate artifact")
		requireNonEmpty(&errs, "governance.attestation.key_env", cfg.Governance.Attestation.KeyEnv, "set the env var name that contains the Ed25519 signing key for attestations")
	}
	for i, sink := range cfg.Integrations.ResultSinks {
		validateResultSink(&errs, fmt.Sprintf("integrations.result_sinks[%d]", i), sink)
	}
	for i, source := range cfg.Integrations.TrendSources {
		validateTrendSource(&errs, fmt.Sprintf("integrations.trend_sources[%d]", i), source)
	}
	for i, summary := range cfg.Integrations.Summaries {
		validateSummary(&errs, fmt.Sprintf("integrations.summaries[%d]", i), summary)
	}

	if errs.HasAny() {
		return errs
	}
	return nil
}

func requireNonEmpty(errs *ValidationErrors, path, value, hint string) {
	if strings.TrimSpace(value) == "" {
		errs.Add(path, "is required", hint)
	}
}

func requirePositiveInt(errs *ValidationErrors, path string, value int, hint string) {
	if value <= 0 {
		errs.Add(path, "must be > 0", hint)
	}
}

func validateAssertion(errs *ValidationErrors, prefix string, assertion core.Assertion) {
	switch strings.TrimSpace(assertion.Type) {
	case "contains", "not_contains":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the text fragment the response should include or exclude")
	case "regex":
		requireNonEmpty(errs, prefix+".pattern", assertion.Pattern, "set a valid Go regular expression to match against the response field")
		if strings.TrimSpace(assertion.Pattern) != "" {
			if _, err := regexp.Compile(assertion.Pattern); err != nil {
				errs.Add(prefix+".pattern", "must be a valid Go regular expression", "fix the pattern syntax or remove the assertion")
			}
		}
	case "json_path":
		requireNonEmpty(errs, prefix+".path", assertion.Path, "set the response path to check, for example response.provider_model or response.body.output.0.content.0.text")
	case "status_code":
		if assertion.IntValue == nil {
			errs.Add(prefix+".int_value", "is required", "set the expected HTTP status code such as 200")
		} else if *assertion.IntValue < 100 || *assertion.IntValue > 599 {
			errs.Add(prefix+".int_value", "must be between 100 and 599", "use a valid HTTP status code")
		}
	case "latency_ms":
		if assertion.IntValue == nil {
			errs.Add(prefix+".int_value", "is required", "set the maximum allowed latency in milliseconds")
		} else if *assertion.IntValue < 0 {
			errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative millisecond threshold")
		}
	case "finish_reason", "tool_call_name":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the expected provider finish reason or tool name")
	case "tool_call_count":
		if assertion.IntValue == nil {
			errs.Add(prefix+".int_value", "is required", "set the expected number of tool calls, including 0 when no tool calls should be present")
		} else if *assertion.IntValue < 0 {
			errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative expected tool call count")
		}
	default:
		errs.Add(prefix+".type", "must be one of contains, not_contains, regex, json_path, status_code, latency_ms, finish_reason, tool_call_count, or tool_call_name", "pick one of the built-in assertion types")
	}

	if severity := strings.TrimSpace(assertion.Severity); severity != "" {
		switch severity {
		case "low", "medium", "high", "critical":
		default:
			errs.Add(prefix+".severity", "must be one of low, medium, high, or critical", "omit severity to use the default, or pick a supported severity level")
		}
	}
}

func validateContextSource(errs *ValidationErrors, prefix string, source core.ContextSource) {
	requireNonEmpty(errs, prefix+".kind", source.Kind, "set the source type, for example retrieved, tool, memory, or approval")
	requireNonEmpty(errs, prefix+".trust", source.Trust, "set the trust tier, for example trusted, untrusted, or approved")
	requireNonEmpty(errs, prefix+".content", source.Content, "set the source content that should be provided to the model during testing")

	switch strings.TrimSpace(source.Kind) {
	case "retrieved", "tool", "memory", "approval":
	case "":
	default:
		errs.Add(prefix+".kind", "must be one of retrieved, tool, memory, or approval", "pick a supported context source kind")
	}

	switch strings.TrimSpace(source.Trust) {
	case "trusted", "untrusted", "approved":
	case "":
	default:
		errs.Add(prefix+".trust", "must be one of trusted, untrusted, or approved", "pick a supported trust tier")
	}
}

func validateMemoryReplaySession(errs *ValidationErrors, prefix string, session core.MemoryReplaySession) {
	requireNonEmpty(errs, prefix+".session_id", session.SessionID, "set the traced session identifier for this ordered replay step")
	if rawSessionID, ok := session.Metadata["session_id"]; ok {
		metadataSessionID := strings.TrimSpace(rawSessionID)
		switch {
		case metadataSessionID == "":
			errs.Add(prefix+".metadata.session_id", "cannot be empty when set", "remove metadata.session_id or set it to the same fixed value as session_id")
		case !strings.EqualFold(metadataSessionID, strings.TrimSpace(session.SessionID)):
			errs.Add(prefix+".metadata.session_id", "must match "+prefix+".session_id", "use one stable traced session_id value for the replay step")
		}
	}
	for i, source := range session.ContextSources {
		validateContextSource(errs, fmt.Sprintf("%s.context_sources[%d]", prefix, i), source)
	}
}

func validateExpectedMutation(errs *ValidationErrors, prefix string, mutation core.ExpectedMutation) {
	requireNonEmpty(errs, prefix+".path", mutation.Path, "set the expected file path that should be created, modified, or deleted")
	requireNonEmpty(errs, prefix+".kind", mutation.Kind, "set the expected mutation kind: created, modified, or deleted")

	switch strings.TrimSpace(mutation.Kind) {
	case "created", "modified", "deleted":
	case "":
	default:
		errs.Add(prefix+".kind", "must be one of created, modified, or deleted", "pick a supported file mutation kind")
	}

	if strings.TrimSpace(mutation.Kind) == "deleted" && strings.TrimSpace(mutation.ContentContains) != "" {
		errs.Add(prefix+".content_contains", "cannot be set when kind is deleted", "remove content_contains for deleted-file expectations")
	}
}

func validateExpectedStateChange(errs *ValidationErrors, prefix string, change core.ExpectedStateChange) {
	if strings.TrimSpace(change.Kind) == "" &&
		strings.TrimSpace(change.Target) == "" &&
		strings.TrimSpace(change.Action) == "" &&
		strings.TrimSpace(change.Status) == "" &&
		strings.TrimSpace(change.SummaryContains) == "" {
		errs.Add(prefix, "must declare at least one selector", "set kind, target, action, status, or summary_contains so cleanr can match an observed state change")
	}
}

func validatePolicyRule(errs *ValidationErrors, prefix string, rule core.PolicyRule) {
	ruleType := strings.TrimSpace(rule.Type)
	mode := strings.TrimSpace(rule.Mode)
	switch ruleType {
	case "tool":
		switch mode {
		case "allow", "deny", "require_approval", "read_only":
		default:
			errs.Add(prefix+".mode", "must be one of allow, deny, require_approval, or read_only", "pick a supported tool policy mode")
		}
		if len(rule.Tools) == 0 {
			errs.Add(prefix+".tools", "must contain at least one tool name", "set the tools this policy rule should match")
		}
	case "state_change":
		switch mode {
		case "allow", "deny", "require_approval":
		default:
			errs.Add(prefix+".mode", "must be one of allow, deny, or require_approval", "pick a supported state-change policy mode")
		}
		if len(rule.StateKinds) == 0 && len(rule.StateActions) == 0 && len(rule.Targets) == 0 {
			errs.Add(prefix, "must declare at least one state selector", "set state_kinds, state_actions, or targets so cleanr can match observed state changes")
		}
	case "trust":
		switch mode {
		case "deny", "require_approval":
		default:
			errs.Add(prefix+".mode", "must be one of deny or require_approval", "pick a supported trust-boundary policy mode")
		}
		if len(rule.Trusts) == 0 {
			errs.Add(prefix+".trusts", "must contain at least one trust tier", "set one or more trust values such as untrusted or approved")
		}
		if len(rule.Tools) == 0 && len(rule.StateKinds) == 0 && len(rule.StateActions) == 0 && len(rule.Targets) == 0 {
			errs.Add(prefix, "must declare an action selector", "set tools and/or state selectors so cleanr knows which actions the trust rule governs")
		}
	case "sink":
		if mode != "approved_only" {
			errs.Add(prefix+".mode", "must be approved_only", "use approved_only to restrict which sink tools may receive sensitive payload")
		}
		if len(rule.ApprovedTools) == 0 {
			errs.Add(prefix+".approved_tools", "must contain at least one tool name", "set the sink tools that are allowed to receive sensitive payload")
		}
	default:
		errs.Add(prefix+".type", "must be one of tool, state_change, trust, or sink", "pick a supported release-policy rule type")
	}

	if severity := strings.TrimSpace(rule.Severity); severity != "" {
		switch severity {
		case "low", "medium", "high", "critical":
		default:
			errs.Add(prefix+".severity", "must be one of low, medium, high, or critical", "omit severity to use the default, or pick a supported severity level")
		}
	}
}

func validateResultSink(errs *ValidationErrors, prefix string, sink core.ResultSinkConfig) {
	switch strings.TrimSpace(sink.Type) {
	case "http", "braintrust", "langfuse", "posthog":
	default:
		errs.Add(prefix+".type", "must be one of http, braintrust, langfuse, or posthog", "use http for a generic JSON webhook, braintrust for a Braintrust-style run publisher, langfuse for a Langfuse trace publisher, or posthog for a PostHog event publisher")
	}
	switch strings.TrimSpace(sink.Type) {
	case "http":
		requireNonEmpty(errs, prefix+".endpoint", sink.Endpoint, "set the remote endpoint that should receive the machine-readable cleanr result payload")
	case "braintrust":
		if strings.TrimSpace(sink.Endpoint) == "" && strings.TrimSpace(sink.Project) == "" {
			errs.Add(prefix, "must set endpoint or project", "set endpoint for a Braintrust-compatible webhook, or set project to use the native Braintrust API connector")
		}
		if strings.TrimSpace(sink.Project) == "" && strings.TrimSpace(sink.Endpoint) != "" {
			// Legacy webhook mode is allowed for backward compatibility.
		}
	case "langfuse":
		requireNonEmpty(errs, prefix+".public_key_env", sink.PublicKeyEnv, "set the env var that contains the Langfuse public key")
		requireNonEmpty(errs, prefix+".secret_key_env", sink.SecretKeyEnv, "set the env var that contains the Langfuse secret key")
	case "posthog":
		requireNonEmpty(errs, prefix+".project_token_env", sink.ProjectTokenEnv, "set the env var that contains the PostHog project API token")
	}
	if rawURL := strings.TrimSpace(sink.Endpoint); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			errs.Add(prefix+".endpoint", "must be an absolute http(s) URL", "use a value such as https://example.internal/cleanr/runs")
		}
	}
	if rawURL := strings.TrimSpace(sink.BaseURL); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			errs.Add(prefix+".base_url", "must be an absolute http(s) URL", "use a value such as https://api.braintrust.dev, https://cloud.langfuse.com, or your self-hosted provider base URL")
		}
	}
	if sink.TimeoutMS < 0 {
		errs.Add(prefix+".timeout_ms", "must be >= 0", "use a non-negative timeout in milliseconds")
	}
}

func validateTrendSource(errs *ValidationErrors, prefix string, source core.TrendSourceConfig) {
	switch strings.TrimSpace(source.Type) {
	case "file":
		requireNonEmpty(errs, prefix+".path", source.Path, "set the path to a retained cleanr trend history file")
	case "http":
		requireNonEmpty(errs, prefix+".url", source.URL, "set the remote URL that returns a cleanr trend history file")
		if rawURL := strings.TrimSpace(source.URL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".url", "must be an absolute http(s) URL", "use a value such as https://example.internal/cleanr/history.json")
			}
		}
	case "braintrust":
		requireNonEmpty(errs, prefix+".project", source.Project, "set the Braintrust project name that stores cleanr release-gate experiments")
		if rawURL := strings.TrimSpace(source.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".base_url", "must be an absolute http(s) URL", "use a value such as https://api.braintrust.dev or your Braintrust data plane URL")
			}
		}
	default:
		errs.Add(prefix+".type", "must be one of file, http, or braintrust", "use file for a retained local artifact, http for a remote history endpoint, or braintrust for native Braintrust experiment history")
	}
	if strings.TrimSpace(source.ViewURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(source.ViewURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			errs.Add(prefix+".view_url", "must be an absolute http(s) URL", "use a direct dashboard URL that reviewers can open when triaging the linked remote experiment")
		}
	}
	if source.HistoryLimit < 0 {
		errs.Add(prefix+".history_limit", "must be >= 0", "use 0 to keep the default remote history window, or set a positive retained-run limit")
	}
	if source.TimeoutMS < 0 {
		errs.Add(prefix+".timeout_ms", "must be >= 0", "use a non-negative timeout in milliseconds")
	}
}

func validateSummary(errs *ValidationErrors, prefix string, summary core.SummaryConfig) {
	switch strings.TrimSpace(summary.Format) {
	case "", "markdown", "json":
	default:
		errs.Add(prefix+".format", "must be one of markdown or json", "use markdown for PR or release notes, or json for downstream automation")
	}
	requireNonEmpty(errs, prefix+".output", summary.Output, "set the output path for the generated PR or release summary artifact")
}
