package types

import (
	"strings"
	"time"
)

type Config struct {
	Version            string                   `json:"version"`
	PolicyPacks        []string                 `json:"policy_packs,omitempty"`
	Plugins            []string                 `json:"plugins,omitempty"`
	Target             TargetConfig             `json:"target"`
	ScenarioGeneration ScenarioGenerationConfig `json:"scenario_generation,omitempty"`
	Scenarios          []Scenario               `json:"scenarios"`
	Suites             SuitesConfig             `json:"suites"`
	Reporting          ReportingConfig          `json:"reporting"`
	Governance         GovernanceConfig         `json:"governance"`
	Integrations       IntegrationsConfig       `json:"integrations,omitempty"`
	ResolvedPlugins    []PluginManifest         `json:"-"`
}

type TargetConfig struct {
	Type            string            `json:"type"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	TimeoutMS       int               `json:"timeout_ms"`
	PromptField     string            `json:"prompt_field"`
	SystemField     string            `json:"system_field"`
	ResponseField   string            `json:"response_field"`
	RequestTemplate any               `json:"request_template"`
	OpenAI          OpenAIConfig      `json:"openai"`
	Anthropic       AnthropicConfig   `json:"anthropic"`
}

type OpenAIConfig struct {
	APIMode      string `json:"api_mode"`
	Model        string `json:"model"`
	APIKeyEnv    string `json:"api_key_env"`
	BaseURL      string `json:"base_url"`
	Organization string `json:"organization"`
	Project      string `json:"project"`
}

type AnthropicConfig struct {
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"`
	BaseURL   string `json:"base_url"`
	Version   string `json:"version"`
	MaxTokens int    `json:"max_tokens"`
}

type ScenarioGenerationConfig struct {
	Enabled       bool                   `json:"enabled"`
	Provider      TargetConfig           `json:"provider"`
	Spec          ScenarioGenerationSpec `json:"spec"`
	OutputFile    string                 `json:"output_file,omitempty"`
	Count         int                    `json:"count,omitempty"`
	RequireReview bool                   `json:"require_review,omitempty"`
}

type ScenarioGenerationSpec struct {
	AppKind      string   `json:"app_kind"`
	Goals        []string `json:"goals,omitempty"`
	RiskAreas    []string `json:"risk_areas,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
}

type Scenario struct {
	Name                 string                `json:"name"`
	System               string                `json:"system"`
	Input                string                `json:"input"`
	Metadata             map[string]string     `json:"metadata"`
	ContextSources       []ContextSource       `json:"context_sources,omitempty"`
	MemoryReplay         []MemoryReplaySession `json:"memory_replay,omitempty"`
	ExpectedMutations    []ExpectedMutation    `json:"expected_mutations,omitempty"`
	ExpectedStateChanges []ExpectedStateChange `json:"expected_state_changes,omitempty"`
	Tags                 []string              `json:"tags"`
	ExpectedContains     []string              `json:"expected_contains"`
	ForbiddenContains    []string              `json:"forbidden_contains"`
	Assertions           []Assertion           `json:"assertions"`
	ReferenceAnswer      string                `json:"reference_answer,omitempty"`
	Rubric               []string              `json:"rubric,omitempty"`
}

type ContextSource struct {
	Name     string            `json:"name,omitempty"`
	Kind     string            `json:"kind"`
	Trust    string            `json:"trust"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type MemoryReplaySession struct {
	Name           string            `json:"name,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	Input          string            `json:"input,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	ContextSources []ContextSource   `json:"context_sources,omitempty"`
}

type ExpectedMutation struct {
	Path            string `json:"path"`
	Kind            string `json:"kind"`
	ContentContains string `json:"content_contains,omitempty"`
}

type ExpectedStateChange struct {
	Kind            string `json:"kind,omitempty"`
	Target          string `json:"target,omitempty"`
	Action          string `json:"action,omitempty"`
	Status          string `json:"status,omitempty"`
	SummaryContains string `json:"summary_contains,omitempty"`
}

type Assertion struct {
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	Value    string `json:"value,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	IntValue *int   `json:"int_value,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`
}

type SuitesConfig struct {
	PromptInjection   PromptInjectionConfig   `json:"prompt_injection"`
	Security          SecurityConfig          `json:"security"`
	Load              LoadConfig              `json:"load"`
	Chaos             ChaosConfig             `json:"chaos"`
	Drift             DriftConfig             `json:"drift"`
	ShadowState       ShadowStateConfig       `json:"shadow_state"`
	Provenance        ProvenanceConfig        `json:"provenance"`
	ClaimTrace        ClaimTraceConfig        `json:"claim_trace"`
	ReleasePolicy     ReleasePolicyConfig     `json:"release_policy"`
	MemorySafety      MemorySafetyConfig      `json:"memory_safety"`
	TokenOptimization TokenOptimizationConfig `json:"token_optimization"`
	LLMJudge          LLMJudgeConfig          `json:"llm_judge"`
}

type PromptInjectionConfig struct {
	Enabled         bool     `json:"enabled"`
	BlockIndicators []string `json:"block_indicators"`
}

type SecurityConfig struct {
	Enabled                  bool     `json:"enabled"`
	LeakPatterns             []string `json:"leak_patterns"`
	MaxPIIMatches            int      `json:"max_pii_matches"`
	DangerousToolIndicators  []string `json:"dangerous_tool_indicators"`
	SecretExposureIndicators []string `json:"secret_exposure_indicators"`
}

type LoadConfig struct {
	Enabled         bool `json:"enabled"`
	VirtualUsers    int  `json:"virtual_users"`
	RequestsPerUser int  `json:"requests_per_user"`
	MaxErrorRatePct int  `json:"max_error_rate_pct"`
	P95LatencyMS    int  `json:"p95_latency_ms"`
}

type ChaosConfig struct {
	Enabled       bool     `json:"enabled"`
	Faults        []string `json:"faults"`
	TimeoutScale  float64  `json:"timeout_scale"`
	NoiseBytes    int      `json:"noise_bytes"`
	MaxErrorRate  int      `json:"max_error_rate_pct"`
	ResponseField string   `json:"response_field"`
}

type DriftConfig struct {
	Enabled                     bool     `json:"enabled"`
	Iterations                  int      `json:"iterations"`
	MaxNormalizedDrift          float64  `json:"max_normalized_drift"`
	MaxSemanticDrift            float64  `json:"max_semantic_drift"`
	MaxSnapshotDrift            float64  `json:"max_snapshot_drift"`
	MaxSemanticSnapshotDrift    float64  `json:"max_semantic_snapshot_drift"`
	BaselineFile                string   `json:"baseline_file"`
	StableTags                  []string `json:"stable_tags"`
	MinConsistencyScore         float64  `json:"min_consistency_score"`
	MinSemanticConsistencyScore float64  `json:"min_semantic_consistency_score"`
}

type ShadowStateConfig struct {
	Enabled           bool     `json:"enabled"`
	Roots             []string `json:"roots"`
	AllowedWritePaths []string `json:"allowed_write_paths"`
}

type ProvenanceConfig struct {
	Enabled                   bool     `json:"enabled"`
	BlockIndicators           []string `json:"block_indicators"`
	ValidationIndicators      []string `json:"validation_indicators"`
	SensitiveIndicators       []string `json:"sensitive_indicators"`
	PrivilegedToolNames       []string `json:"privileged_tool_names"`
	ApprovalRequiredToolNames []string `json:"approval_required_tool_names"`
	ApprovedSinkToolNames     []string `json:"approved_sink_tool_names"`
}

type ClaimTraceConfig struct {
	Enabled               bool     `json:"enabled"`
	CitationIndicators    []string `json:"citation_indicators"`
	ToolClaimIndicators   []string `json:"tool_claim_indicators"`
	ApprovalIndicators    []string `json:"approval_indicators"`
	StateChangeIndicators []string `json:"state_change_indicators"`
}

type ReleasePolicyConfig struct {
	Enabled             bool         `json:"enabled"`
	SensitiveIndicators []string     `json:"sensitive_indicators"`
	ReadOnlyIndicators  []string     `json:"read_only_indicators"`
	MutatingIndicators  []string     `json:"mutating_indicators"`
	Rules               []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Name          string   `json:"name,omitempty"`
	Type          string   `json:"type"`
	Mode          string   `json:"mode"`
	Tools         []string `json:"tools,omitempty"`
	StateKinds    []string `json:"state_kinds,omitempty"`
	StateActions  []string `json:"state_actions,omitempty"`
	Targets       []string `json:"targets,omitempty"`
	Trusts        []string `json:"trusts,omitempty"`
	ApprovedTools []string `json:"approved_tools,omitempty"`
	Indicators    []string `json:"indicators,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	Message       string   `json:"message,omitempty"`
}

type MemorySafetyConfig struct {
	Enabled bool `json:"enabled"`
}

type TokenOptimizationConfig struct {
	Enabled                     bool    `json:"enabled"`
	MaxInputTokens              int     `json:"max_input_tokens"`
	MaxOutputTokens             int     `json:"max_output_tokens"`
	MaxTotalTokens              int     `json:"max_total_tokens"`
	MaxOutputInputRatio         float64 `json:"max_output_input_ratio"`
	MaxPromptDuplicationRatio   float64 `json:"max_prompt_duplication_ratio"`
	MaxResponseDuplicationRatio float64 `json:"max_response_duplication_ratio"`
	SuggestedMaxOutputTokens    int     `json:"suggested_max_output_tokens"`
}

// LLMJudgeConfig configures the model-graded "llm_judge" suite. A separate
// judge model scores each target response against a rubric (and an optional
// per-scenario reference answer) on a 1..Scale Likert scale. The case passes
// when the aggregated normalized score meets MinScore. Self-consistency
// sampling (Samples > 1) gates out judges that disagree with themselves.
type LLMJudgeConfig struct {
	Enabled          bool         `json:"enabled"`
	Mode             string       `json:"mode,omitempty"`
	Provider         TargetConfig `json:"provider"`
	Baseline         TargetConfig `json:"baseline,omitempty"`
	Criteria         []string     `json:"criteria,omitempty"`
	Scale            int          `json:"scale,omitempty"`
	MinScore         float64      `json:"min_score,omitempty"`
	MinWinRate       float64      `json:"min_win_rate,omitempty"`
	Samples          int          `json:"samples,omitempty"`
	MaxDisagreement  float64      `json:"max_disagreement,omitempty"`
	RequireReference bool         `json:"require_reference,omitempty"`
	StableTags       []string     `json:"stable_tags,omitempty"`
}

// ModeValue returns the grading mode, defaulting to rubric "score" grading.
// "pairwise" compares the target under test against Baseline.
func (c LLMJudgeConfig) ModeValue() string {
	switch m := strings.ToLower(strings.TrimSpace(c.Mode)); m {
	case "pairwise":
		return "pairwise"
	default:
		return "score"
	}
}

// ScaleValue returns the configured Likert ceiling, defaulting to 5.
func (c LLMJudgeConfig) ScaleValue() int {
	if c.Scale <= 1 {
		return 5
	}
	return c.Scale
}

// SamplesValue returns the configured self-consistency sample count,
// defaulting to a single judge call.
func (c LLMJudgeConfig) SamplesValue() int {
	if c.Samples <= 0 {
		return 1
	}
	return c.Samples
}

type ReportingConfig struct {
	Format             string          `json:"format"`
	Output             string          `json:"output"`
	TrendFile          string          `json:"trend_file"`
	ReplayArtifactFile string          `json:"replay_artifact_file"`
	TrendLimit         int             `json:"trend_limit"`
	BuildID            string          `json:"build_id"`
	TrendGates         TrendGateConfig `json:"trend_gates"`
}

type GovernanceConfig struct {
	Attestation AttestationConfig `json:"attestation"`
}

type AttestationConfig struct {
	Enabled bool   `json:"enabled"`
	Output  string `json:"output"`
	KeyEnv  string `json:"key_env"`
	KeyID   string `json:"key_id"`
}

type IntegrationsConfig struct {
	ResultSinks  []ResultSinkConfig  `json:"result_sinks,omitempty"`
	TrendSources []TrendSourceConfig `json:"trend_sources,omitempty"`
	Summaries    []SummaryConfig     `json:"summaries,omitempty"`
}

type ResultSinkConfig struct {
	Name            string            `json:"name,omitempty"`
	Type            string            `json:"type"`
	BaseURL         string            `json:"base_url,omitempty"`
	Endpoint        string            `json:"endpoint,omitempty"`
	APIKeyEnv       string            `json:"api_key_env,omitempty"`
	ProjectTokenEnv string            `json:"project_token_env,omitempty"`
	PublicKeyEnv    string            `json:"public_key_env,omitempty"`
	SecretKeyEnv    string            `json:"secret_key_env,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Project         string            `json:"project,omitempty"`
	Experiment      string            `json:"experiment,omitempty"`
	RunURLTemplate  string            `json:"run_url_template,omitempty"`
	IncludeReplay   bool              `json:"include_replay_artifact,omitempty"`
	IncludeAttest   bool              `json:"include_attestation,omitempty"`
	TimeoutMS       int               `json:"timeout_ms,omitempty"`
}

type TrendSourceConfig struct {
	Name         string            `json:"name,omitempty"`
	Type         string            `json:"type"`
	BaseURL      string            `json:"base_url,omitempty"`
	Path         string            `json:"path,omitempty"`
	URL          string            `json:"url,omitempty"`
	APIKeyEnv    string            `json:"api_key_env,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Project      string            `json:"project,omitempty"`
	Experiment   string            `json:"experiment,omitempty"`
	HistoryLimit int               `json:"history_limit,omitempty"`
	ViewURL      string            `json:"view_url,omitempty"`
	TimeoutMS    int               `json:"timeout_ms,omitempty"`
}

type SummaryConfig struct {
	Name   string `json:"name,omitempty"`
	Format string `json:"format,omitempty"`
	Output string `json:"output"`
}

type PluginManifest struct {
	Name          string               `json:"name"`
	Version       string               `json:"version,omitempty"`
	PolicyPacks   []string             `json:"policy_packs,omitempty"`
	Suites        []PluginSuite        `json:"suites,omitempty"`
	StateAdapters []PluginStateAdapter `json:"state_adapters,omitempty"`
}

type PluginSuite struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int               `json:"timeout_ms,omitempty"`
}

type PluginStateAdapter struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int               `json:"timeout_ms,omitempty"`
}

type TrendGateConfig struct {
	Preset                        string   `json:"preset,omitempty"`
	Enabled                       bool     `json:"enabled"`
	RequiredWindow                int      `json:"required_window"`
	MaxFailedSuitesDelta          *int     `json:"max_failed_suites_delta,omitempty"`
	MaxFailedCasesDelta           *int     `json:"max_failed_cases_delta,omitempty"`
	MaxDurationIncreasePct        *float64 `json:"max_duration_increase_pct,omitempty"`
	MaxSemanticDriftDelta         *float64 `json:"max_semantic_drift_delta,omitempty"`
	MaxBaselineSemanticDriftDelta *float64 `json:"max_baseline_semantic_drift_delta,omitempty"`
	FailOnRegressedSuites         bool     `json:"fail_on_regressed_suites,omitempty"`
}

func (c TargetConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutMS) * time.Millisecond
}

func (c TargetConfig) TargetType() string {
	if strings.TrimSpace(c.Type) == "" {
		return "http"
	}
	return strings.ToLower(strings.TrimSpace(c.Type))
}

func (c OpenAIConfig) APIModeValue() string {
	if strings.TrimSpace(c.APIMode) == "" {
		return "responses"
	}
	return strings.ToLower(strings.TrimSpace(c.APIMode))
}

func (c AnthropicConfig) VersionValue() string {
	if strings.TrimSpace(c.Version) == "" {
		return "2023-06-01"
	}
	return strings.TrimSpace(c.Version)
}

func (c AnthropicConfig) MaxTokensValue() int {
	if c.MaxTokens <= 0 {
		return 1024
	}
	return c.MaxTokens
}
