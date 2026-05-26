package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func ValidateConfig(cfg core.Config) error {
	var errs ValidationErrors

	validateCoreConfig(&errs, cfg)
	validateSuitesConfig(&errs, cfg)
	validateReportingConfig(&errs, cfg.Reporting)
	validateGovernanceConfig(&errs, cfg.Governance)
	validateIntegrationsConfig(&errs, cfg.Integrations)

	if errs.HasAny() {
		return errs
	}
	return nil
}

func validateCoreConfig(errs *ValidationErrors, cfg core.Config) {
	validateTargetConfig(errs, "target", cfg.Target)
	validateScenarioGenerationConfig(errs, cfg.ScenarioGeneration)
	validateTargetTimeout(errs, cfg.Target)
	validateScenarios(errs, cfg.Scenarios, cfg.ScenarioGeneration.Enabled)
}

func validateScenarioGenerationConfig(errs *ValidationErrors, cfg core.ScenarioGenerationConfig) {
	if !cfg.Enabled {
		return
	}
	if strings.TrimSpace(cfg.Provider.Type) == "" {
		errs.Add("scenario_generation.provider.type", "is required", "set scenario_generation.provider.type to openai, anthropic, or http")
	}
	validateTargetConfig(errs, "scenario_generation.provider", cfg.Provider)
	if cfg.Provider.TimeoutMS < 0 {
		errs.Add("scenario_generation.provider.timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
	}
	requireNonEmpty(errs, "scenario_generation.spec.app_kind", cfg.Spec.AppKind, "set the app kind being tested, for example support-assistant or release-bot")
	if len(cfg.Spec.Goals) == 0 {
		errs.Add("scenario_generation.spec.goals", "at least one goal is required", "add one or more goals such as refund policy, account recovery, or release approval")
	}
	if len(cfg.Spec.RiskAreas) == 0 {
		errs.Add("scenario_generation.spec.risk_areas", "at least one risk area is required", "add one or more risk areas such as prompt injection, pii leakage, or unsafe tool use")
	}
	if cfg.Count <= 0 {
		errs.Add("scenario_generation.count", "must be >= 1", "set the number of generated scenarios to a positive integer")
	}
	requireNonEmpty(errs, "scenario_generation.output_file", cfg.OutputFile, "set a persisted dataset path such as generated/cleanr.dataset.yaml so generated scenarios can be reviewed")
}

func validateTargetTimeout(errs *ValidationErrors, cfg core.TargetConfig) {
	if cfg.TimeoutMS < 0 {
		errs.Add("target.timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
	}
}

func validateScenarios(errs *ValidationErrors, scenarios []core.Scenario, scenarioGenerationEnabled bool) {
	scenarioNames := make(map[string]int, len(scenarios))
	if len(scenarios) == 0 && !scenarioGenerationEnabled {
		errs.Add("scenarios", "at least one scenario is required", "add a scenario with both name and input so cleanr has something to execute")
	}
	for i, scenario := range scenarios {
		validateScenario(errs, i, scenario, scenarioNames)
	}
}

func validateScenario(errs *ValidationErrors, index int, scenario core.Scenario, scenarioNames map[string]int) {
	prefix := fmt.Sprintf("scenarios[%d]", index)
	requireNonEmpty(errs, prefix+".name", scenario.Name, "set a short stable scenario name, for example \"happy-path\"")
	requireNonEmpty(errs, prefix+".input", scenario.Input, "set the end-user prompt or test input for this scenario")
	validateScenarioName(errs, prefix, index, scenario.Name, scenarioNames)
	validateScenarioContextSources(errs, prefix, scenario.ContextSources)
	validateScenarioMemoryReplay(errs, prefix, scenario.MemoryReplay)
	validateScenarioExpectedMutations(errs, prefix, scenario.ExpectedMutations)
	validateScenarioExpectedStateChanges(errs, prefix, scenario.ExpectedStateChanges)
	validateScenarioAssertions(errs, prefix, scenario.Assertions)
}

func validateScenarioName(errs *ValidationErrors, prefix string, index int, name string, scenarioNames map[string]int) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return
	}
	if first, ok := scenarioNames[trimmed]; ok {
		errs.Add(prefix+".name", fmt.Sprintf("duplicates scenarios[%d].name", first), "rename duplicate scenarios so each scenario name is unique in reports")
		return
	}
	scenarioNames[trimmed] = index
}

func validateScenarioContextSources(errs *ValidationErrors, prefix string, sources []core.ContextSource) {
	for i, source := range sources {
		validateContextSource(errs, fmt.Sprintf("%s.context_sources[%d]", prefix, i), source)
	}
}

func validateScenarioMemoryReplay(errs *ValidationErrors, prefix string, replay []core.MemoryReplaySession) {
	if len(replay) == 1 {
		errs.Add(prefix+".memory_replay", "must contain at least two sessions", "add a second ordered session so memory replay can be evaluated across a session boundary")
	}
	replaySessionIDs := make(map[string]int, len(replay))
	for i, session := range replay {
		sessionPrefix := fmt.Sprintf("%s.memory_replay[%d]", prefix, i)
		validateMemoryReplaySession(errs, sessionPrefix, session)
		validateReplaySessionID(errs, prefix, sessionPrefix, i, session.SessionID, replaySessionIDs)
	}
}

func validateReplaySessionID(errs *ValidationErrors, prefix, sessionPrefix string, index int, sessionID string, replaySessionIDs map[string]int) {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return
	}
	if first, ok := replaySessionIDs[trimmed]; ok {
		errs.Add(sessionPrefix+".session_id", fmt.Sprintf("duplicates %s.memory_replay[%d].session_id", prefix, first), "set a unique traced session_id for each replay step")
		return
	}
	replaySessionIDs[trimmed] = index
}

func validateScenarioExpectedMutations(errs *ValidationErrors, prefix string, mutations []core.ExpectedMutation) {
	for i, mutation := range mutations {
		validateExpectedMutation(errs, fmt.Sprintf("%s.expected_mutations[%d]", prefix, i), mutation)
	}
}

func validateScenarioExpectedStateChanges(errs *ValidationErrors, prefix string, changes []core.ExpectedStateChange) {
	for i, change := range changes {
		validateExpectedStateChange(errs, fmt.Sprintf("%s.expected_state_changes[%d]", prefix, i), change)
	}
}

func validateScenarioAssertions(errs *ValidationErrors, prefix string, assertions []core.Assertion) {
	for i, assertion := range assertions {
		validateAssertion(errs, fmt.Sprintf("%s.assertions[%d]", prefix, i), assertion)
	}
}

func validateSuitesConfig(errs *ValidationErrors, cfg core.Config) {
	validateLoadSuite(errs, cfg.Suites.Load)
	validateSecuritySuite(errs, cfg.Suites.Security)
	validateChaosSuite(errs, cfg.Suites.Chaos)
	validateDriftSuite(errs, cfg.Suites.Drift)
	validateShadowStateSuite(errs, cfg.Suites.ShadowState)
	validateProvenanceSuite(errs, cfg.Suites.Provenance)
	validateClaimTraceSuite(errs, cfg.Suites.ClaimTrace)
	validateReleasePolicySuite(errs, cfg.Suites.ReleasePolicy)
	validateTokenOptimizationSuite(errs, cfg.Suites.TokenOptimization)
}

func validateLoadSuite(errs *ValidationErrors, cfg core.LoadConfig) {
	if !cfg.Enabled {
		return
	}
	requirePositiveInt(errs, "suites.load.virtual_users", cfg.VirtualUsers, "set virtual_users to at least 1 when the load suite is enabled")
	requirePositiveInt(errs, "suites.load.requests_per_user", cfg.RequestsPerUser, "set requests_per_user to at least 1 when the load suite is enabled")
	if cfg.MaxErrorRatePct < 0 || cfg.MaxErrorRatePct > 100 {
		errs.Add("suites.load.max_error_rate_pct", "must be between 0 and 100", "use a whole-number percentage such as 5 or 25")
	}
	if cfg.P95LatencyMS < 0 {
		errs.Add("suites.load.p95_latency_ms", "must be >= 0", "set a positive latency budget in milliseconds")
	}
}

func validateSecuritySuite(errs *ValidationErrors, cfg core.SecurityConfig) {
	for i, pattern := range cfg.LeakPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			errs.Add(fmt.Sprintf("suites.security.leak_patterns[%d]", i), "must be a valid Go regular expression", "fix the pattern syntax or remove the entry if it is no longer needed")
		}
	}
}

func validateChaosSuite(errs *ValidationErrors, cfg core.ChaosConfig) {
	if !cfg.Enabled {
		return
	}
	validateChaosFaults(errs, cfg.Faults)
	if cfg.TimeoutScale <= 0 {
		errs.Add("suites.chaos.timeout_scale", "must be > 0", "use a fractional multiplier such as 0.4 to shorten the timeout")
	}
	if cfg.NoiseBytes < 0 {
		errs.Add("suites.chaos.noise_bytes", "must be >= 0", "set a non-negative number of injected bytes")
	}
	if cfg.MaxErrorRate < 0 || cfg.MaxErrorRate > 100 {
		errs.Add("suites.chaos.max_error_rate_pct", "must be between 0 and 100", "use a whole-number percentage such as 35")
	}
}

func validateChaosFaults(errs *ValidationErrors, faults []string) {
	allowedFaults := map[string]struct{}{
		"tight_deadline":   {},
		"context_overflow": {},
		"duplicate_turn":   {},
	}
	for i, fault := range faults {
		if _, ok := allowedFaults[fault]; !ok {
			errs.Add(fmt.Sprintf("suites.chaos.faults[%d]", i), "must be one of tight_deadline, context_overflow, or duplicate_turn", "replace the value with a supported built-in chaos fault")
		}
	}
}

func validateDriftSuite(errs *ValidationErrors, cfg core.DriftConfig) {
	if !cfg.Enabled {
		return
	}
	if cfg.Iterations < 2 {
		errs.Add("suites.drift.iterations", "must be >= 2", "set iterations to 2 or more so drift can compare repeated runs")
	}
	validateUnitInterval(errs, "suites.drift.max_normalized_drift", cfg.MaxNormalizedDrift, "use a decimal threshold such as 0.3")
	validateUnitInterval(errs, "suites.drift.max_semantic_drift", cfg.MaxSemanticDrift, "use a decimal threshold such as 0.25")
	validateUnitInterval(errs, "suites.drift.max_snapshot_drift", cfg.MaxSnapshotDrift, "use a decimal threshold such as 0.18")
	validateUnitInterval(errs, "suites.drift.max_semantic_snapshot_drift", cfg.MaxSemanticSnapshotDrift, "use a decimal threshold such as 0.2")
	validateUnitInterval(errs, "suites.drift.min_consistency_score", cfg.MinConsistencyScore, "use a decimal threshold such as 0.7")
	validateUnitInterval(errs, "suites.drift.min_semantic_consistency_score", cfg.MinSemanticConsistencyScore, "use a decimal threshold such as 0.75")
}

func validateShadowStateSuite(errs *ValidationErrors, cfg core.ShadowStateConfig) {
	if !cfg.Enabled {
		return
	}
	if len(cfg.Roots) == 0 {
		errs.Add("suites.shadow_state.roots", "must contain at least one path", "set one or more files or directories that cleanr should snapshot before and after each run")
	}
	for i, root := range cfg.Roots {
		requireNonEmpty(errs, fmt.Sprintf("suites.shadow_state.roots[%d]", i), root, "set a file or directory path to observe for side effects")
	}
	for i, path := range cfg.AllowedWritePaths {
		requireNonEmpty(errs, fmt.Sprintf("suites.shadow_state.allowed_write_paths[%d]", i), path, "set a file or directory path where mutations are allowed")
	}
}

func validateProvenanceSuite(errs *ValidationErrors, cfg core.ProvenanceConfig) {
	if !cfg.Enabled {
		return
	}
	validateStringList(errs, "suites.provenance.block_indicators", cfg.BlockIndicators, "set a non-empty refusal marker")
	validateStringList(errs, "suites.provenance.validation_indicators", cfg.ValidationIndicators, "set a non-empty validation marker")
	validateStringList(errs, "suites.provenance.sensitive_indicators", cfg.SensitiveIndicators, "set a non-empty sensitive-data marker")
	validateStringList(errs, "suites.provenance.privileged_tool_names", cfg.PrivilegedToolNames, "set a non-empty tool name")
	validateStringList(errs, "suites.provenance.approval_required_tool_names", cfg.ApprovalRequiredToolNames, "set a non-empty tool name")
	validateStringList(errs, "suites.provenance.approved_sink_tool_names", cfg.ApprovedSinkToolNames, "set a non-empty tool name")
}

func validateClaimTraceSuite(errs *ValidationErrors, cfg core.ClaimTraceConfig) {
	if !cfg.Enabled {
		return
	}
	validateStringList(errs, "suites.claim_trace.citation_indicators", cfg.CitationIndicators, "set a non-empty citation marker")
	validateStringList(errs, "suites.claim_trace.tool_claim_indicators", cfg.ToolClaimIndicators, "set a non-empty tool-claim marker")
	validateStringList(errs, "suites.claim_trace.approval_indicators", cfg.ApprovalIndicators, "set a non-empty approval marker")
	validateStringList(errs, "suites.claim_trace.state_change_indicators", cfg.StateChangeIndicators, "set a non-empty state-change marker")
}

func validateReleasePolicySuite(errs *ValidationErrors, cfg core.ReleasePolicyConfig) {
	if !cfg.Enabled {
		return
	}
	validateStringList(errs, "suites.release_policy.sensitive_indicators", cfg.SensitiveIndicators, "set a non-empty sensitive-data marker")
	validateStringList(errs, "suites.release_policy.read_only_indicators", cfg.ReadOnlyIndicators, "set a non-empty read-only marker")
	validateStringList(errs, "suites.release_policy.mutating_indicators", cfg.MutatingIndicators, "set a non-empty mutating marker")
	for i, rule := range cfg.Rules {
		validatePolicyRule(errs, fmt.Sprintf("suites.release_policy.rules[%d]", i), rule)
	}
}

func validateTokenOptimizationSuite(errs *ValidationErrors, cfg core.TokenOptimizationConfig) {
	if !cfg.Enabled {
		return
	}
	validateNonNegativeInt(errs, "suites.token_optimization.max_input_tokens", cfg.MaxInputTokens, "set a non-negative token budget or omit the field to use the default")
	validateNonNegativeInt(errs, "suites.token_optimization.max_output_tokens", cfg.MaxOutputTokens, "set a non-negative token budget or omit the field to use the default")
	validateNonNegativeInt(errs, "suites.token_optimization.max_total_tokens", cfg.MaxTotalTokens, "set a non-negative token budget or omit the field to use the default")
	validatePositiveFloat(errs, "suites.token_optimization.max_output_input_ratio", cfg.MaxOutputInputRatio, "use a positive ratio such as 1.4")
	validateUnitInterval(errs, "suites.token_optimization.max_prompt_duplication_ratio", cfg.MaxPromptDuplicationRatio, "use a decimal ratio such as 0.18")
	validateUnitInterval(errs, "suites.token_optimization.max_response_duplication_ratio", cfg.MaxResponseDuplicationRatio, "use a decimal ratio such as 0.12")
	validateNonNegativeInt(errs, "suites.token_optimization.suggested_max_output_tokens", cfg.SuggestedMaxOutputTokens, "set a non-negative suggestion or omit the field to use the default")
}

func validateReportingConfig(errs *ValidationErrors, cfg core.ReportingConfig) {
	if format := strings.TrimSpace(cfg.Format); format != "" {
		switch format {
		case "text", "json", "junit", "sarif":
		default:
			errs.Add("reporting.format", "must be one of text, json, junit, or sarif", "use one of the built-in report formats or omit the field for text output")
		}
	}
	validateNonNegativeInt(errs, "reporting.trend_limit", cfg.TrendLimit, "use 0 to disable history trimming or set a positive run-retention count such as 30")
	if !isValidTrendGatePreset(cfg.TrendGates.Preset) {
		errs.Add("reporting.trend_gates.preset", "must be one of strict, moderate, or exploratory", "choose a built-in trend gate preset or remove the field to set thresholds manually")
	}
	if !cfg.TrendGates.Enabled {
		return
	}
	if strings.TrimSpace(cfg.TrendFile) == "" {
		errs.Add("reporting.trend_file", "is required when trend gates are enabled", "set reporting.trend_file so cleanr can compare the current run to prior retained runs")
	}
	if cfg.TrendGates.RequiredWindow < 2 {
		errs.Add("reporting.trend_gates.required_window", "must be >= 2", "use at least 2 retained runs so cleanr can compare the current build against a previous one")
	}
	validateOptionalNonNegativeInt(errs, "reporting.trend_gates.max_failed_suites_delta", cfg.TrendGates.MaxFailedSuitesDelta, "use a non-negative number of additional failed suites allowed between builds")
	validateOptionalNonNegativeInt(errs, "reporting.trend_gates.max_failed_cases_delta", cfg.TrendGates.MaxFailedCasesDelta, "use a non-negative number of additional failed cases allowed between builds")
	validateOptionalNonNegativeFloat(errs, "reporting.trend_gates.max_duration_increase_pct", cfg.TrendGates.MaxDurationIncreasePct, "use a non-negative percentage such as 20 for a 20 percent duration increase budget")
	validateOptionalUnitInterval(errs, "reporting.trend_gates.max_semantic_drift_delta", cfg.TrendGates.MaxSemanticDriftDelta, "use a decimal drift delta such as 0.08")
	validateOptionalUnitInterval(errs, "reporting.trend_gates.max_baseline_semantic_drift_delta", cfg.TrendGates.MaxBaselineSemanticDriftDelta, "use a decimal drift delta such as 0.05")
}

func validateGovernanceConfig(errs *ValidationErrors, cfg core.GovernanceConfig) {
	if !cfg.Attestation.Enabled {
		return
	}
	requireNonEmpty(errs, "governance.attestation.output", cfg.Attestation.Output, "set the attestation output path so cleanr can write the signed release-gate artifact")
	requireNonEmpty(errs, "governance.attestation.key_env", cfg.Attestation.KeyEnv, "set the env var name that contains the Ed25519 signing key for attestations")
}

func validateIntegrationsConfig(errs *ValidationErrors, cfg core.IntegrationsConfig) {
	for i, sink := range cfg.ResultSinks {
		validateResultSink(errs, fmt.Sprintf("integrations.result_sinks[%d]", i), sink)
	}
	for i, source := range cfg.TrendSources {
		validateTrendSource(errs, fmt.Sprintf("integrations.trend_sources[%d]", i), source)
	}
	for i, summary := range cfg.Summaries {
		validateSummary(errs, fmt.Sprintf("integrations.summaries[%d]", i), summary)
	}
}

func validateTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	urlHint := "set the full API endpoint URL"
	promptHint := "set the request field that receives the prompt text"
	responseHint := "set the JSON path that contains the model text response"
	if prefix == "target" {
		urlHint = "set target.url to the full API endpoint URL"
		promptHint = "set target.prompt_field to the request field that receives the prompt text"
		responseHint = "set target.response_field to the JSON path that contains the model text response"
	}
	switch cfg.TargetType() {
	case "http":
		requireNonEmpty(errs, prefix+".url", cfg.URL, urlHint)
		requireNonEmpty(errs, prefix+".prompt_field", cfg.PromptField, promptHint)
		requireNonEmpty(errs, prefix+".response_field", cfg.ResponseField, responseHint)
		if rawURL := strings.TrimSpace(cfg.URL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".url", "must be an absolute http(s) URL", "use a value such as http://localhost:8080/v1/chat or https://api.example.com/v1/chat")
			}
		}
	case "openai":
		requireNonEmpty(errs, prefix+".openai.model", cfg.OpenAI.Model, "set the OpenAI model name, for example gpt-4o-mini or gpt-4.1-mini")
		switch cfg.OpenAI.APIModeValue() {
		case "responses", "chat_completions":
		default:
			errs.Add(prefix+".openai.api_mode", "must be one of responses or chat_completions", "use responses for new projects or chat_completions for legacy-compatible message requests")
		}
		if rawURL := strings.TrimSpace(cfg.OpenAI.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".openai.base_url", "must be an absolute http(s) URL", "use a value such as https://api.openai.com/v1 or a compatible base URL for testing")
			}
		}
	case "anthropic":
		requireNonEmpty(errs, prefix+".anthropic.model", cfg.Anthropic.Model, "set the Anthropic model name, for example claude-sonnet-4-20250514")
		if rawURL := strings.TrimSpace(cfg.Anthropic.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".anthropic.base_url", "must be an absolute http(s) URL", "use a value such as https://api.anthropic.com/v1 or a compatible base URL for testing")
			}
		}
		if cfg.Anthropic.MaxTokens < 0 {
			errs.Add(prefix+".anthropic.max_tokens", "must be >= 0", "set a positive max_tokens budget or omit the field to use the default")
		}
	default:
		errs.Add(prefix+".type", "must be one of http, openai, or anthropic", "set the target type to http, openai, or anthropic")
	}
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

func validateStringList(errs *ValidationErrors, path string, values []string, hint string) {
	for i, value := range values {
		requireNonEmpty(errs, fmt.Sprintf("%s[%d]", path, i), value, hint)
	}
}

func validateNonNegativeInt(errs *ValidationErrors, path string, value int, hint string) {
	if value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validatePositiveFloat(errs *ValidationErrors, path string, value float64, hint string) {
	if value <= 0 {
		errs.Add(path, "must be > 0", hint)
	}
}

func validateOptionalNonNegativeInt(errs *ValidationErrors, path string, value *int, hint string) {
	if value != nil && *value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validateNonNegativeFloat(errs *ValidationErrors, path string, value float64, hint string) {
	if value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validateOptionalNonNegativeFloat(errs *ValidationErrors, path string, value *float64, hint string) {
	if value != nil && *value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validateUnitInterval(errs *ValidationErrors, path string, value float64, hint string) {
	if value < 0 || value > 1 {
		errs.Add(path, "must be between 0 and 1", hint)
	}
}

func validateOptionalUnitInterval(errs *ValidationErrors, path string, value *float64, hint string) {
	if value != nil && (*value < 0 || *value > 1) {
		errs.Add(path, "must be between 0 and 1", hint)
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
		validateToolPolicyRule(errs, prefix, mode, rule)
	case "state_change":
		validateStateChangePolicyRule(errs, prefix, mode, rule)
	case "trust":
		validateTrustPolicyRule(errs, prefix, mode, rule)
	case "sink":
		validateSinkPolicyRule(errs, prefix, mode, rule)
	default:
		errs.Add(prefix+".type", "must be one of tool, state_change, trust, or sink", "pick a supported release-policy rule type")
	}

	validatePolicySeverity(errs, prefix, rule.Severity)
}

func validateToolPolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if !matchesLiteral(mode, "allow", "deny", "require_approval", "read_only") {
		errs.Add(prefix+".mode", "must be one of allow, deny, require_approval, or read_only", "pick a supported tool policy mode")
	}
	if len(rule.Tools) == 0 {
		errs.Add(prefix+".tools", "must contain at least one tool name", "set the tools this policy rule should match")
	}
}

func validateStateChangePolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if !matchesLiteral(mode, "allow", "deny", "require_approval") {
		errs.Add(prefix+".mode", "must be one of allow, deny, or require_approval", "pick a supported state-change policy mode")
	}
	if len(rule.StateKinds) == 0 && len(rule.StateActions) == 0 && len(rule.Targets) == 0 {
		errs.Add(prefix, "must declare at least one state selector", "set state_kinds, state_actions, or targets so cleanr can match observed state changes")
	}
}

func validateTrustPolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if !matchesLiteral(mode, "deny", "require_approval") {
		errs.Add(prefix+".mode", "must be one of deny or require_approval", "pick a supported trust-boundary policy mode")
	}
	if len(rule.Trusts) == 0 {
		errs.Add(prefix+".trusts", "must contain at least one trust tier", "set one or more trust values such as untrusted or approved")
	}
	if len(rule.Tools) == 0 && len(rule.StateKinds) == 0 && len(rule.StateActions) == 0 && len(rule.Targets) == 0 {
		errs.Add(prefix, "must declare an action selector", "set tools and/or state selectors so cleanr knows which actions the trust rule governs")
	}
}

func validateSinkPolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if mode != "approved_only" {
		errs.Add(prefix+".mode", "must be approved_only", "use approved_only to restrict which sink tools may receive sensitive payload")
	}
	if len(rule.ApprovedTools) == 0 {
		errs.Add(prefix+".approved_tools", "must contain at least one tool name", "set the sink tools that are allowed to receive sensitive payload")
	}
}

func validatePolicySeverity(errs *ValidationErrors, prefix, severity string) {
	trimmed := strings.TrimSpace(severity)
	if trimmed == "" {
		return
	}
	if !matchesLiteral(trimmed, "low", "medium", "high", "critical") {
		errs.Add(prefix+".severity", "must be one of low, medium, high, or critical", "omit severity to use the default, or pick a supported severity level")
	}
}

func matchesLiteral(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
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
