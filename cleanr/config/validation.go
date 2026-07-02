package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/adapters"
	"github.com/devr-tools/cleanr/cleanr/core"
)

// supportedTargetTypesList renders adapters.SupportedTargetTypes as a
// human-readable, comma-separated list with an Oxford "or" before the last
// element, for embedding in validation hint strings.
func supportedTargetTypesList() string {
	types := adapters.SupportedTargetTypes
	switch len(types) {
	case 0:
		return ""
	case 1:
		return types[0]
	}
	return strings.Join(types[:len(types)-1], ", ") + ", or " + types[len(types)-1]
}

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
	validateOpenAPIConfig(errs, cfg)
	validateTargetTimeout(errs, cfg.Target)
	validateScenarios(errs, cfg.Scenarios, cfg.ScenarioGeneration.Enabled)
}

func validateScenarioGenerationConfig(errs *ValidationErrors, cfg core.ScenarioGenerationConfig) {
	if !cfg.Enabled {
		return
	}
	if strings.TrimSpace(cfg.Provider.Type) == "" {
		errs.Add("scenario_generation.provider.type", "is required", "set scenario_generation.provider.type to "+supportedTargetTypesList())
	}
	validateTargetConfig(errs, "scenario_generation.provider", cfg.Provider)
	if cfg.Provider.TimeoutMS < 0 {
		errs.Add("scenario_generation.provider.timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
	}
	requireNonEmpty(errs, "scenario_generation.spec.app_kind", cfg.Spec.AppKind, "set the app kind being tested, for example support-assistant or release-bot")
	switch strings.ToLower(strings.TrimSpace(cfg.Spec.Mode)) {
	case "", "standard", "adversarial":
	default:
		errs.Add("scenario_generation.spec.mode", "must be one of standard or adversarial", "use standard for routine coverage generation or adversarial for red-team scenario generation")
	}
	if len(cfg.Spec.Goals) == 0 {
		errs.Add("scenario_generation.spec.goals", "at least one goal is required", "add one or more goals such as refund policy, account recovery, or release approval")
	}
	if len(cfg.Spec.RiskAreas) == 0 {
		errs.Add("scenario_generation.spec.risk_areas", "at least one risk area is required", "add one or more risk areas such as prompt injection, pii leakage, or unsafe tool use")
	}
	if cfg.Spec.ModeValue() == "adversarial" {
		if len(cfg.Spec.AttackFamilies) == 0 {
			errs.Add("scenario_generation.spec.attack_families", "at least one attack family is required in adversarial mode", "add one or more adversarial families such as prompt injection, jailbreak, tool abuse, or data exfiltration")
		}
		validateStringList(errs, "scenario_generation.spec.attack_families", cfg.Spec.AttackFamilies, "remove empty attack-family values or set a concrete adversarial family")
	}
	if cfg.Count <= 0 {
		errs.Add("scenario_generation.count", "must be >= 1", "set the number of generated scenarios to a positive integer")
	}
	requireNonEmpty(errs, "scenario_generation.output_file", cfg.OutputFile, "set a persisted dataset path such as generated/cleanr.dataset.yaml so generated scenarios can be reviewed")
}

func validateOpenAPIConfig(errs *ValidationErrors, cfg core.Config) {
	if cfg.Target.OpenAPI.Enabled && cfg.Target.TargetType() != "http" {
		errs.Add("target.openapi.enabled", "is only supported for http targets", "disable target.openapi.enabled or switch target.type to http")
	}
	openapiCfg := cfg.OpenAPI
	if !openapiCfg.ScenarioGenerationEnabled() && !openapiCfg.ContractDiffEnabled() {
		return
	}
	validateOpenAPISource(errs, "openapi.source", openapiCfg.Source, "set openapi.source.path, openapi.source.url, or openapi.source.inline to the candidate OpenAPI 3 document")
	if openapiCfg.ScenarioGenerationEnabled() {
		if cfg.Target.TargetType() != "http" {
			errs.Add("openapi.scenario_generation", "requires target.type http", "use an http target so generated OpenAPI scenarios can map onto concrete REST operations")
		}
		validateStringList(errs, "openapi.scenario_generation.include_tags", openapiCfg.ScenarioGeneration.IncludeTags, "remove empty tag names or set concrete OpenAPI tags to include")
		validateOpenAPIMethodList(errs, "openapi.scenario_generation.include_methods", openapiCfg.ScenarioGeneration.IncludeMethods)
		requireNonEmpty(errs, "openapi.scenario_generation.output_file", openapiCfg.ScenarioGeneration.OutputFile, "set a dataset path such as generated/openapi.scenarios.yaml when persisting generated scenarios")
	}
	if openapiCfg.ContractDiffEnabled() {
		validateOpenAPISource(errs, "openapi.contract_diff.baseline", openapiCfg.ContractDiff.Baseline, "set openapi.contract_diff.baseline.path, .url, or .inline to the baseline OpenAPI document")
		requireNonEmpty(errs, "openapi.contract_diff.output_file", openapiCfg.ContractDiff.OutputFile, "set an artifact path such as reports/openapi.diff.yaml when persisting the contract diff")
	}
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
	if len(scenario.Turns) == 0 && !scenarioHasMultimodalInputs(scenario) && !scenarioHasOpenAPIRequest(scenario) {
		requireNonEmpty(errs, prefix+".input", scenario.Input, "set the end-user prompt or test input for this scenario")
	}
	validateScenarioName(errs, prefix, index, scenario.Name, scenarioNames)
	validateScenarioMediaInputs(errs, prefix, scenario.Images, scenario.Audio, scenario.PDFs)
	validateScenarioJudgeOutputs(errs, prefix, scenario.JudgeOutputs)
	validateScenarioTurns(errs, prefix, scenario.Turns)
	validateScenarioContextSources(errs, prefix, scenario.ContextSources)
	validateScenarioMemoryReplay(errs, prefix, scenario.MemoryReplay)
	validateScenarioExpectedMutations(errs, prefix, scenario.ExpectedMutations)
	validateScenarioExpectedStateChanges(errs, prefix, scenario.ExpectedStateChanges)
	validateScenarioAssertions(errs, prefix, scenario.Assertions)
	if len(scenario.Turns) > 0 && len(scenario.MemoryReplay) > 0 {
		errs.Add(prefix, "cannot combine turns with memory_replay", "use transcript turns for multi-turn request evaluation, or memory_replay for suite-specific memory checks")
	}
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

func validateScenarioTurns(errs *ValidationErrors, prefix string, turns []core.ConversationTurn) {
	for i, turn := range turns {
		validateConversationTurn(errs, fmt.Sprintf("%s.turns[%d]", prefix, i), turn)
	}
}

func validateScenarioMediaInputs(errs *ValidationErrors, prefix string, images, audio, pdfs []core.MediaInput) {
	for i, item := range images {
		validateMediaInput(errs, fmt.Sprintf("%s.images[%d]", prefix, i), "image", item)
	}
	for i, item := range audio {
		validateMediaInput(errs, fmt.Sprintf("%s.audio[%d]", prefix, i), "audio", item)
	}
	for i, item := range pdfs {
		validateMediaInput(errs, fmt.Sprintf("%s.pdfs[%d]", prefix, i), "pdf", item)
	}
}

func validateScenarioJudgeOutputs(errs *ValidationErrors, prefix string, outputs []core.JudgeOutput) {
	for i, item := range outputs {
		validateJudgeOutput(errs, fmt.Sprintf("%s.judge_outputs[%d]", prefix, i), item)
	}
}

func scenarioHasMultimodalInputs(scenario core.Scenario) bool {
	return len(scenario.Images) > 0 || len(scenario.Audio) > 0 || len(scenario.PDFs) > 0
}

func scenarioHasOpenAPIRequest(scenario core.Scenario) bool {
	if len(scenario.Metadata) == 0 {
		return false
	}
	return strings.TrimSpace(scenario.Metadata["openapi.path"]) != "" || strings.TrimSpace(scenario.Metadata["openapi.method"]) != ""
}

func validateScenarioMemoryReplay(errs *ValidationErrors, prefix string, replay []core.MemoryReplaySession) {
	if len(replay) == 1 {
		errs.Add(prefix+".memory_replay", "must contain at least two sessions", "add a second ordered session so memory replay can be evaluated across a session boundary")
	}
	replaySessionIDs := make(map[string]int, len(replay))
	for i, session := range replay {
		ref := replaySessionRef{
			prefix:        prefix,
			sessionPrefix: fmt.Sprintf("%s.memory_replay[%d]", prefix, i),
			index:         i,
			sessionID:     session.SessionID,
		}
		validateMemoryReplaySession(errs, ref.sessionPrefix, session)
		validateReplaySessionID(errs, replaySessionIDs, ref)
	}
}

type replaySessionRef struct {
	prefix        string
	sessionPrefix string
	index         int
	sessionID     string
}

func validateReplaySessionID(errs *ValidationErrors, replaySessionIDs map[string]int, ref replaySessionRef) {
	trimmed := strings.TrimSpace(ref.sessionID)
	if trimmed == "" {
		return
	}
	if first, ok := replaySessionIDs[trimmed]; ok {
		errs.Add(ref.sessionPrefix+".session_id", fmt.Sprintf("duplicates %s.memory_replay[%d].session_id", ref.prefix, first), "set a unique traced session_id for each replay step")
		return
	}
	replaySessionIDs[trimmed] = ref.index
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
	validateLLMJudgeSuite(errs, cfg.Suites.LLMJudge, cfg.Scenarios)
}

func validateLLMJudgeSuite(errs *ValidationErrors, cfg core.LLMJudgeConfig, scenarios []core.Scenario) {
	if !cfg.Enabled {
		return
	}
	validateLLMJudgeProvider(errs, cfg.Provider, "suites.llm_judge.provider", "set suites.llm_judge.provider.type to "+supportedTargetTypesList()+" so a judge model can grade responses")
	validateLLMJudgeMode(errs, cfg)
	validateLLMJudgeThresholds(errs, cfg)
	validateLLMJudgeTargets(errs, cfg.Ensemble, "suites.llm_judge.ensemble")
	validateLLMJudgeTargets(errs, cfg.ComparisonTargets, "suites.llm_judge.comparison_targets")
	validateLLMJudgeCalibration(errs, cfg)
	validateLLMJudgeReferences(errs, cfg, scenarios)
}

func validateLLMJudgeProvider(errs *ValidationErrors, provider core.TargetConfig, prefix, requiredHint string) {
	if strings.TrimSpace(provider.Type) == "" {
		errs.Add(prefix+".type", "is required", requiredHint)
	}
	validateTargetConfig(errs, prefix, provider)
	if provider.TimeoutMS < 0 {
		errs.Add(prefix+".timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
	}
}

func validateLLMJudgeMode(errs *ValidationErrors, cfg core.LLMJudgeConfig) {
	switch m := strings.ToLower(strings.TrimSpace(cfg.Mode)); m {
	case "", "score", "pairwise":
	default:
		errs.Add("suites.llm_judge.mode", "must be one of score or pairwise", "use score for rubric grading or pairwise to compare the target against a baseline")
	}
	if cfg.ModeValue() != "pairwise" {
		return
	}
	validateLLMJudgeProvider(errs, cfg.Baseline, "suites.llm_judge.baseline", "set suites.llm_judge.baseline.type to "+supportedTargetTypesList()+" so the target can be compared against a baseline")
	if cfg.MinWinRate < 0 || cfg.MinWinRate > 1 {
		errs.Add("suites.llm_judge.min_win_rate", "must be between 0 and 1", "use a fraction of decisive comparisons the target must win, such as 0.5")
	}
}

func validateLLMJudgeThresholds(errs *ValidationErrors, cfg core.LLMJudgeConfig) {
	if cfg.Scale != 0 && cfg.Scale < 2 {
		errs.Add("suites.llm_judge.scale", "must be >= 2", "use a Likert ceiling such as 5, or omit the field to use the default")
	}
	if cfg.MinScore < 0 || cfg.MinScore > 1 {
		errs.Add("suites.llm_judge.min_score", "must be between 0 and 1", "use a normalized pass threshold such as 0.6 (3 out of 5)")
	}
	if cfg.Samples < 0 {
		errs.Add("suites.llm_judge.samples", "must be >= 0", "set the self-consistency sample count to a positive integer or omit the field to use a single judge call")
	}
	validateUnitInterval(errs, "suites.llm_judge.confidence_level", cfg.ConfidenceLevel, "use a confidence level such as 0.95 for Wilson pass-rate intervals")
	validateUnitInterval(errs, "suites.llm_judge.min_pass_rate", cfg.MinPassRate, "use a fractional pass-rate gate such as 0.9 for a 9/10 reliability threshold")
	validateUnitInterval(errs, "suites.llm_judge.max_flake_rate", cfg.MaxFlakeRate, "use a fractional instability budget such as 0.1")
	validateUnitInterval(errs, "suites.llm_judge.max_disagreement", cfg.MaxDisagreement, "use a normalized spread such as 0.4")
	validateUnitInterval(errs, "suites.llm_judge.cascade_margin", cfg.CascadeMargin, "use a fractional margin such as 0.1 so extra judges only run near the pass threshold")
	validateUnitInterval(errs, "suites.llm_judge.min_calibration_accuracy", cfg.MinCalibrationAccuracy, "use a fraction such as 0.8 to require agreement with human labels")
	validateNonNegativeFloat(errs, "suites.llm_judge.max_calibration_mae", cfg.MaxCalibrationMAE, "use a non-negative mean absolute error threshold such as 0.15")
}

func validateLLMJudgeTargets(errs *ValidationErrors, targets []core.TargetConfig, prefix string) {
	for i, target := range targets {
		targetPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		validateTargetConfig(errs, targetPrefix, target)
		if target.TimeoutMS < 0 {
			errs.Add(targetPrefix+".timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
		}
	}
}

func validateLLMJudgeCalibration(errs *ValidationErrors, cfg core.LLMJudgeConfig) {
	if (cfg.MinCalibrationAccuracy > 0 || cfg.MaxCalibrationMAE > 0) && strings.TrimSpace(cfg.CalibrationFile) == "" {
		errs.Add("suites.llm_judge.calibration_file", "is required when calibration gates are enabled", "set a JSON or YAML label file so judge outputs can be checked against human ratings")
	}
}

func validateLLMJudgeReferences(errs *ValidationErrors, cfg core.LLMJudgeConfig, scenarios []core.Scenario) {
	if !cfg.RequireReference {
		return
	}
	for _, idx := range judgeScopedScenarios(scenarios, cfg.StableTags) {
		if strings.TrimSpace(scenarios[idx].ReferenceAnswer) == "" {
			errs.Add(fmt.Sprintf("scenarios[%d].reference_answer", idx), "is required when suites.llm_judge.require_reference is true", "add a reference_answer for this scenario or disable require_reference")
		}
	}
}

// judgeScopedScenarios returns the indexes of scenarios graded by the judge
// suite, honoring the optional stable-tag filter.
func judgeScopedScenarios(scenarios []core.Scenario, stableTags []string) []int {
	out := make([]int, 0, len(scenarios))
	if len(stableTags) == 0 {
		for i := range scenarios {
			out = append(out, i)
		}
		return out
	}
	want := make(map[string]struct{}, len(stableTags))
	for _, tag := range stableTags {
		want[tag] = struct{}{}
	}
	for i, scenario := range scenarios {
		for _, tag := range scenario.Tags {
			if _, ok := want[tag]; ok {
				out = append(out, i)
				break
			}
		}
	}
	return out
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
	validateNonNegativeFloat(errs, "suites.load.max_cost_per_request", cfg.MaxCostPerRequest, "set a non-negative per-request budget such as 0.01, or omit the field to disable the gate")
	validateNonNegativeFloat(errs, "suites.load.input_cost_per_1m_tokens", cfg.InputCostPer1MTokens, "set a non-negative input token price per 1M tokens when cost-per-request gating is enabled")
	validateNonNegativeFloat(errs, "suites.load.output_cost_per_1m_tokens", cfg.OutputCostPer1MTokens, "set a non-negative output token price per 1M tokens when cost-per-request gating is enabled")
	if cfg.MinTokensPerSecond < 0 {
		errs.Add("suites.load.min_tokens_per_second", "must be >= 0", "set a non-negative aggregate token throughput floor, or omit the field to disable the gate")
	}
	if cfg.MaxCostPerRequest > 0 && cfg.InputCostPer1MTokens == 0 && cfg.OutputCostPer1MTokens == 0 {
		errs.Add("suites.load", "cost-per-request gating requires pricing", "set suites.load.input_cost_per_1m_tokens and/or suites.load.output_cost_per_1m_tokens so cleanr can estimate request cost from provider usage")
	}
	validateStringList(errs, "suites.load.scenario_tags", cfg.ScenarioTags, "set one or more non-empty tags to scope the load profile, or omit the field to use every scenario")
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
	validateUnitInterval(errs, "suites.drift.confidence_level", cfg.ConfidenceLevel, "use a confidence level such as 0.95 for Wilson pass-rate intervals")
	validateUnitInterval(errs, "suites.drift.min_pass_rate", cfg.MinPassRate, "use a fractional pass-rate gate such as 0.9 for a 9/10 stability threshold")
	validateUnitInterval(errs, "suites.drift.max_flake_rate", cfg.MaxFlakeRate, "use a fractional instability budget such as 0.1")
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
		case "text", "json", "junit", "sarif", "agent", "html":
		default:
			errs.Add("reporting.format", "must be one of text, json, junit, sarif, agent, or html", "use one of the built-in report formats or omit the field for text output")
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
