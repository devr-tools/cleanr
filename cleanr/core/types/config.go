package types

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Version            string                   `json:"version"`
	PolicyPacks        []string                 `json:"policy_packs,omitempty"`
	Plugins            []string                 `json:"plugins,omitempty"`
	Concurrency        int                      `json:"concurrency,omitempty"`
	Target             TargetConfig             `json:"target"`
	ScenarioGeneration ScenarioGenerationConfig `json:"scenario_generation,omitempty"`
	OpenAPI            OpenAPIConfig            `json:"openapi,omitempty"`
	Scenarios          []Scenario               `json:"scenarios"`
	Suites             SuitesConfig             `json:"suites"`
	Reporting          ReportingConfig          `json:"reporting"`
	Governance         GovernanceConfig         `json:"governance"`
	Integrations       IntegrationsConfig       `json:"integrations,omitempty"`
	ResolvedPlugins    []PluginManifest         `json:"-"`
}

type TargetConfig struct {
	Type            string              `json:"type"`
	Name            string              `json:"name"`
	URL             string              `json:"url"`
	Method          string              `json:"method"`
	Stream          bool                `json:"stream,omitempty"`
	Headers         map[string]string   `json:"headers"`
	TimeoutMS       int                 `json:"timeout_ms"`
	PromptField     string              `json:"prompt_field"`
	SystemField     string              `json:"system_field"`
	ResponseField   string              `json:"response_field"`
	RequestTemplate any                 `json:"request_template"`
	CLI             CLIConfig           `json:"cli"`
	GraphQL         GraphQLConfig       `json:"graphql"`
	GRPC            GRPCConfig          `json:"grpc"`
	OpenAI          OpenAIConfig        `json:"openai"`
	OpenAPI         OpenAPITargetConfig `json:"openapi,omitempty"`
	Anthropic       AnthropicConfig     `json:"anthropic"`
	MCP             MCPConfig           `json:"mcp"`
}

type CLIConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type GraphQLConfig struct {
	Query             string `json:"query"`
	OperationName     string `json:"operation_name,omitempty"`
	VariablesTemplate any    `json:"variables_template,omitempty"`
}

type GRPCConfig struct {
	Address   string `json:"address,omitempty"`
	Method    string `json:"method,omitempty"`
	Plaintext bool   `json:"plaintext,omitempty"`
}

type OpenAIConfig struct {
	APIMode       string `json:"api_mode"`
	Model         string `json:"model"`
	APIKeyEnv     string `json:"api_key_env"`
	BaseURL       string `json:"base_url"`
	APIVersion    string `json:"api_version,omitempty"`
	Organization  string `json:"organization"`
	Project       string `json:"project"`
	Provider      string `json:"provider,omitempty"`
	AuthHeader    string `json:"auth_header,omitempty"`
	AuthScheme    string `json:"auth_scheme,omitempty"`
	QuirksProfile string `json:"quirks_profile,omitempty"`
}

type AnthropicConfig struct {
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"`
	BaseURL   string `json:"base_url"`
	Version   string `json:"version"`
	MaxTokens int    `json:"max_tokens"`
}

type MCPConfig struct {
	URL               string            `json:"url"`
	Tool              string            `json:"tool"`
	Initialize        bool              `json:"initialize,omitempty"`
	ResultTextPath    string            `json:"result_text_path,omitempty"`
	ArgumentsTemplate any               `json:"arguments_template,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
}

type ScenarioGenerationConfig struct {
	Enabled       bool                   `json:"enabled"`
	Provider      TargetConfig           `json:"provider"`
	Spec          ScenarioGenerationSpec `json:"spec"`
	OutputFile    string                 `json:"output_file,omitempty"`
	Count         int                    `json:"count,omitempty"`
	RequireReview *bool                  `json:"require_review,omitempty"`
}

// RequireReviewValue reports whether generated scenarios need human review
// before use. It defaults to true; a pointer keeps an explicit
// `require_review: false` expressible instead of being silently inverted.
func (c ScenarioGenerationConfig) RequireReviewValue() bool {
	if c.RequireReview != nil {
		return *c.RequireReview
	}
	return true
}

type ScenarioGenerationSpec struct {
	AppKind        string   `json:"app_kind"`
	Mode           string   `json:"mode,omitempty"`
	Goals          []string `json:"goals,omitempty"`
	RiskAreas      []string `json:"risk_areas,omitempty"`
	AttackFamilies []string `json:"attack_families,omitempty"`
	Instructions   string   `json:"instructions,omitempty"`
}

type OpenAPIConfig struct {
	Source             OpenAPISource                   `json:"source,omitempty"`
	ScenarioGeneration OpenAPIScenarioGenerationConfig `json:"scenario_generation,omitempty"`
	ContractDiff       OpenAPIContractDiffConfig       `json:"contract_diff,omitempty"`
}

type OpenAPISource struct {
	Path   string `json:"path,omitempty"`
	URL    string `json:"url,omitempty"`
	Inline any    `json:"inline,omitempty"`
}

type OpenAPIScenarioGenerationConfig struct {
	Enabled           bool     `json:"enabled,omitempty"`
	OutputFile        string   `json:"output_file,omitempty"`
	IncludeTags       []string `json:"include_tags,omitempty"`
	IncludeMethods    []string `json:"include_methods,omitempty"`
	IncludeDeprecated bool     `json:"include_deprecated,omitempty"`
}

type OpenAPIContractDiffConfig struct {
	Enabled        bool          `json:"enabled,omitempty"`
	Baseline       OpenAPISource `json:"baseline,omitempty"`
	OutputFile     string        `json:"output_file,omitempty"`
	FailOnBreaking bool          `json:"fail_on_breaking,omitempty"`
}

type OpenAPITargetConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

type OpenAPIContractDiff struct {
	Breaking bool                       `json:"breaking,omitempty"`
	Summary  OpenAPIContractDiffSummary `json:"summary"`
	Changes  []OpenAPIContractChange    `json:"changes,omitempty"`
}

type OpenAPIContractDiffSummary struct {
	BreakingChanges    int `json:"breaking_changes,omitempty"`
	NonBreakingChanges int `json:"non_breaking_changes,omitempty"`
	OperationsAdded    int `json:"operations_added,omitempty"`
	OperationsRemoved  int `json:"operations_removed,omitempty"`
	OperationsChanged  int `json:"operations_changed,omitempty"`
}

type OpenAPIContractChange struct {
	Kind        string `json:"kind"`
	Level       string `json:"level,omitempty"`
	Method      string `json:"method,omitempty"`
	Path        string `json:"path,omitempty"`
	OperationID string `json:"operation_id,omitempty"`
	Location    string `json:"location,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

type Scenario struct {
	Name                 string                `json:"name"`
	System               string                `json:"system"`
	Input                string                `json:"input"`
	Images               []MediaInput          `json:"images,omitempty"`
	Audio                []MediaInput          `json:"audio,omitempty"`
	PDFs                 []MediaInput          `json:"pdfs,omitempty"`
	JudgeOutputs         []JudgeOutput         `json:"judge_outputs,omitempty"`
	Turns                []ConversationTurn    `json:"turns,omitempty"`
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

type MediaInput struct {
	URL       string `json:"url,omitempty"`
	Path      string `json:"path,omitempty"`
	Data      string `json:"data,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Caption   string `json:"caption,omitempty"`
}

type JudgeOutput struct {
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	Path      string `json:"path,omitempty"`
	Value     string `json:"value,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

type ConversationTurn struct {
	Role            string           `json:"role"`
	Content         string           `json:"content"`
	Images          []MediaInput     `json:"images,omitempty"`
	Audio           []MediaInput     `json:"audio,omitempty"`
	PDFs            []MediaInput     `json:"pdfs,omitempty"`
	MockToolResults []MockToolResult `json:"mock_tool_results,omitempty"`
	Name            string           `json:"name,omitempty"`
	ToolCallID      string           `json:"tool_call_id,omitempty"`
}

type MockToolResult struct {
	Name       string       `json:"name"`
	Arguments  string       `json:"arguments,omitempty"`
	Content    string       `json:"content"`
	Images     []MediaInput `json:"images,omitempty"`
	Audio      []MediaInput `json:"audio,omitempty"`
	PDFs       []MediaInput `json:"pdfs,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
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
	Schema   any    `json:"schema,omitempty"`
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
	Enabled               bool     `json:"enabled"`
	VirtualUsers          int      `json:"virtual_users"`
	RequestsPerUser       int      `json:"requests_per_user"`
	MaxErrorRatePct       int      `json:"max_error_rate_pct"`
	P95LatencyMS          int      `json:"p95_latency_ms"`
	MaxCostPerRequest     float64  `json:"max_cost_per_request,omitempty"`
	MinTokensPerSecond    float64  `json:"min_tokens_per_second,omitempty"`
	InputCostPer1MTokens  float64  `json:"input_cost_per_1m_tokens,omitempty"`
	OutputCostPer1MTokens float64  `json:"output_cost_per_1m_tokens,omitempty"`
	ScenarioTags          []string `json:"scenario_tags,omitempty"`
}

type ChaosConfig struct {
	Enabled       bool     `json:"enabled"`
	Faults        []string `json:"faults"`
	TimeoutScale  float64  `json:"timeout_scale"`
	NoiseBytes    int      `json:"noise_bytes"`
	MaxErrorRate  int      `json:"max_error_rate_pct"`
	ResponseField string   `json:"response_field"`
}

// DriftConfig thresholds are pointers so an explicit zero ("no drift
// tolerated at all") survives default application instead of being replaced
// by the default; use the *Value accessors to read them.
type DriftConfig struct {
	Enabled                     bool     `json:"enabled"`
	Iterations                  int      `json:"iterations"`
	MaxNormalizedDrift          *float64 `json:"max_normalized_drift,omitempty"`
	MaxSemanticDrift            *float64 `json:"max_semantic_drift,omitempty"`
	MaxSnapshotDrift            *float64 `json:"max_snapshot_drift,omitempty"`
	MaxSemanticSnapshotDrift    *float64 `json:"max_semantic_snapshot_drift,omitempty"`
	BaselineFile                string   `json:"baseline_file"`
	StableTags                  []string `json:"stable_tags"`
	MinConsistencyScore         *float64 `json:"min_consistency_score,omitempty"`
	MinSemanticConsistencyScore *float64 `json:"min_semantic_consistency_score,omitempty"`
	ConfidenceLevel             float64  `json:"confidence_level,omitempty"`
	MinPassRate                 float64  `json:"min_pass_rate,omitempty"`
	MaxFlakeRate                float64  `json:"max_flake_rate,omitempty"`
}

// MaxNormalizedDriftValue returns the lexical drift ceiling, defaulting to 0.3.
func (c DriftConfig) MaxNormalizedDriftValue() float64 {
	if c.MaxNormalizedDrift != nil {
		return *c.MaxNormalizedDrift
	}
	return 0.3
}

// MaxSemanticDriftValue returns the semantic drift ceiling, defaulting to 0.25.
func (c DriftConfig) MaxSemanticDriftValue() float64 {
	if c.MaxSemanticDrift != nil {
		return *c.MaxSemanticDrift
	}
	return 0.25
}

// MaxSnapshotDriftValue returns the baseline drift ceiling, defaulting to the
// lexical drift ceiling.
func (c DriftConfig) MaxSnapshotDriftValue() float64 {
	if c.MaxSnapshotDrift != nil {
		return *c.MaxSnapshotDrift
	}
	return c.MaxNormalizedDriftValue()
}

// MaxSemanticSnapshotDriftValue returns the semantic baseline drift ceiling,
// defaulting to the semantic drift ceiling.
func (c DriftConfig) MaxSemanticSnapshotDriftValue() float64 {
	if c.MaxSemanticSnapshotDrift != nil {
		return *c.MaxSemanticSnapshotDrift
	}
	return c.MaxSemanticDriftValue()
}

// MinConsistencyScoreValue returns the lexical consistency floor, defaulting
// to 0.7.
func (c DriftConfig) MinConsistencyScoreValue() float64 {
	if c.MinConsistencyScore != nil {
		return *c.MinConsistencyScore
	}
	return 0.7
}

// MinSemanticConsistencyScoreValue returns the semantic consistency floor,
// defaulting to 0.75.
func (c DriftConfig) MinSemanticConsistencyScoreValue() float64 {
	if c.MinSemanticConsistencyScore != nil {
		return *c.MinSemanticConsistencyScore
	}
	return 0.75
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
	Enabled                bool           `json:"enabled"`
	Mode                   string         `json:"mode,omitempty"`
	Provider               TargetConfig   `json:"provider"`
	Baseline               TargetConfig   `json:"baseline,omitempty"`
	Criteria               []string       `json:"criteria,omitempty"`
	Scale                  int            `json:"scale,omitempty"`
	MinScore               *float64       `json:"min_score,omitempty"`
	MinWinRate             float64        `json:"min_win_rate,omitempty"`
	Samples                int            `json:"samples,omitempty"`
	MaxDisagreement        float64        `json:"max_disagreement,omitempty"`
	ConfidenceLevel        float64        `json:"confidence_level,omitempty"`
	MinPassRate            float64        `json:"min_pass_rate,omitempty"`
	MaxFlakeRate           float64        `json:"max_flake_rate,omitempty"`
	RequireReference       bool           `json:"require_reference,omitempty"`
	StableTags             []string       `json:"stable_tags,omitempty"`
	Ensemble               []TargetConfig `json:"ensemble,omitempty"`
	CascadeMargin          float64        `json:"cascade_margin,omitempty"`
	ComparisonTargets      []TargetConfig `json:"comparison_targets,omitempty"`
	CalibrationFile        string         `json:"calibration_file,omitempty"`
	MinCalibrationAccuracy float64        `json:"min_calibration_accuracy,omitempty"`
	MaxCalibrationMAE      float64        `json:"max_calibration_mae,omitempty"`
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

// MinScoreValue returns the normalized pass threshold, defaulting to 0.6. A
// pointer keeps an explicit `min_score: 0` (no floor) expressible.
func (c LLMJudgeConfig) MinScoreValue() float64 {
	if c.MinScore != nil {
		return *c.MinScore
	}
	return 0.6
}

// SamplesValue returns the configured self-consistency sample count,
// defaulting to a single judge call.
func (c LLMJudgeConfig) SamplesValue() int {
	if c.Samples <= 0 {
		return 1
	}
	return c.Samples
}

func (c LLMJudgeConfig) ConfidenceLevelValue() float64 {
	if c.ConfidenceLevel <= 0 {
		return 0.95
	}
	return c.ConfidenceLevel
}

func (c DriftConfig) ConfidenceLevelValue() float64 {
	if c.ConfidenceLevel <= 0 {
		return 0.95
	}
	return c.ConfidenceLevel
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
	Source        string               `json:"source,omitempty"`
	BaseDir       string               `json:"base_dir,omitempty"`
	Runtime       PluginRuntimeConfig  `json:"runtime,omitempty"`
	PolicyPacks   []string             `json:"policy_packs,omitempty"`
	Suites        []PluginSuite        `json:"suites,omitempty"`
	StateAdapters []PluginStateAdapter `json:"state_adapters,omitempty"`
	Probes        []PluginProbe        `json:"probes,omitempty"`
}

type PluginRuntimeConfig struct {
	Backend    string `json:"backend,omitempty"`
	Entrypoint string `json:"entrypoint,omitempty"`
}

type PluginSuite struct {
	Name      string              `json:"name"`
	Command   string              `json:"command"`
	Args      []string            `json:"args,omitempty"`
	Env       map[string]string   `json:"env,omitempty"`
	Runtime   PluginRuntimeConfig `json:"runtime,omitempty"`
	TimeoutMS int                 `json:"timeout_ms,omitempty"`
}

type PluginStateAdapter struct {
	Name      string              `json:"name"`
	Command   string              `json:"command"`
	Args      []string            `json:"args,omitempty"`
	Env       map[string]string   `json:"env,omitempty"`
	Runtime   PluginRuntimeConfig `json:"runtime,omitempty"`
	TimeoutMS int                 `json:"timeout_ms,omitempty"`
}

type PluginProbe struct {
	Name      string              `json:"name"`
	Kind      string              `json:"kind,omitempty"`
	Command   string              `json:"command"`
	Args      []string            `json:"args,omitempty"`
	Env       map[string]string   `json:"env,omitempty"`
	Runtime   PluginRuntimeConfig `json:"runtime,omitempty"`
	TimeoutMS int                 `json:"timeout_ms,omitempty"`
}

type DBProbeObservation struct {
	Engine    string         `json:"engine,omitempty"`
	Database  string         `json:"database,omitempty"`
	Table     string         `json:"table,omitempty"`
	Operation string         `json:"operation,omitempty"`
	Status    string         `json:"status,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Count     int            `json:"count,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type QueueProbeObservation struct {
	Provider  string         `json:"provider,omitempty"`
	Queue     string         `json:"queue,omitempty"`
	Topic     string         `json:"topic,omitempty"`
	Operation string         `json:"operation,omitempty"`
	Status    string         `json:"status,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	MessageID string         `json:"message_id,omitempty"`
	Depth     int            `json:"depth,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type TrendGateConfig struct {
	Preset                        string   `json:"preset,omitempty"`
	Enabled                       *bool    `json:"enabled,omitempty"`
	RequiredWindow                int      `json:"required_window"`
	MaxFailedSuitesDelta          *int     `json:"max_failed_suites_delta,omitempty"`
	MaxFailedCasesDelta           *int     `json:"max_failed_cases_delta,omitempty"`
	MaxDurationIncreasePct        *float64 `json:"max_duration_increase_pct,omitempty"`
	MaxSemanticDriftDelta         *float64 `json:"max_semantic_drift_delta,omitempty"`
	MaxBaselineSemanticDriftDelta *float64 `json:"max_baseline_semantic_drift_delta,omitempty"`
	FailOnRegressedSuites         bool     `json:"fail_on_regressed_suites,omitempty"`
}

// EnabledValue reports whether trend gates are active, defaulting to false. A
// pointer keeps an explicit `enabled:` distinguishable from unset, so presets
// can supply a default without overriding the user's choice.
func (c TrendGateConfig) EnabledValue() bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return false
}

// defaultTargetTimeoutMS mirrors the request timeout applied by the config
// package's applyDefaults. SDK-built configs never run applyDefaults, so
// Timeout() falls back to this value when TimeoutMS is unset to avoid handing
// callers a zero-length deadline (which expires immediately).
const defaultTargetTimeoutMS = 5000

func (c TargetConfig) Timeout() time.Duration {
	ms := c.TimeoutMS
	if ms <= 0 {
		ms = defaultTargetTimeoutMS
	}
	return time.Duration(ms) * time.Millisecond
}

// defaultCaseConcurrency bounds the per-scenario worker pool used by the
// read-heavy engines when the config does not specify a value.
const defaultCaseConcurrency = 4

// CaseConcurrency returns the bounded worker-pool size engines use when
// invoking the target for independent scenarios. It is configurable via the
// top-level "concurrency" field and defaults to a safe value when unset.
func (c Config) CaseConcurrency() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return defaultCaseConcurrency
}

func (c OpenAPIConfig) HasSource() bool {
	return strings.TrimSpace(c.Source.Path) != "" || strings.TrimSpace(c.Source.URL) != "" || c.Source.Inline != nil
}

func (c OpenAPIConfig) ScenarioGenerationEnabled() bool {
	return c.ScenarioGeneration.Enabled
}

func (c OpenAPIConfig) ContractDiffEnabled() bool {
	return c.ContractDiff.Enabled
}

func (c ScenarioGenerationSpec) ModeValue() string {
	if strings.ToLower(strings.TrimSpace(c.Mode)) == "adversarial" {
		return "adversarial"
	}
	return "standard"
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

func (c OpenAIConfig) ProviderValue(targetType string) string {
	if provider := strings.TrimSpace(c.Provider); provider != "" {
		return strings.ToLower(provider)
	}
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "openai_compatible":
		return "openai_compatible"
	case "azure_openai", "gemini", "bedrock", "vertex", "mistral":
		return strings.ToLower(strings.TrimSpace(targetType))
	}
	return "openai"
}

func (c OpenAIConfig) AuthHeaderValue() string {
	if header := strings.TrimSpace(c.AuthHeader); header != "" {
		return header
	}
	return "Authorization"
}

func (c OpenAIConfig) AuthSchemeValue() string {
	if scheme := strings.TrimSpace(c.AuthScheme); strings.EqualFold(scheme, "none") {
		return ""
	} else if scheme != "" {
		return scheme
	}
	return "Bearer"
}

func (c OpenAIConfig) APIKeyEnvValue(targetType string) string {
	if env := strings.TrimSpace(c.APIKeyEnv); env != "" {
		return env
	}
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "azure_openai":
		return "AZURE_OPENAI_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	case "bedrock":
		return "BEDROCK_API_KEY"
	case "vertex":
		return "VERTEX_AI_ACCESS_TOKEN"
	case "mistral":
		return "MISTRAL_API_KEY"
	default:
		return "OPENAI_API_KEY"
	}
}

func (c OpenAIConfig) BaseURLValue(targetType string) string {
	if base := strings.TrimSpace(c.BaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	case "mistral":
		return "https://api.mistral.ai/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

func (c OpenAIConfig) APIVersionValue() string {
	return strings.TrimSpace(c.APIVersion)
}

func (c OpenAIConfig) AuthHeaderForTarget(targetType string) string {
	if header := strings.TrimSpace(c.AuthHeader); header != "" {
		return header
	}
	if strings.EqualFold(strings.TrimSpace(targetType), "azure_openai") {
		return "api-key"
	}
	return "Authorization"
}

func (c OpenAIConfig) AuthSchemeForTarget(targetType string) string {
	if scheme := strings.TrimSpace(c.AuthScheme); strings.EqualFold(scheme, "none") {
		return ""
	} else if scheme != "" {
		return scheme
	}
	if strings.EqualFold(strings.TrimSpace(targetType), "azure_openai") {
		return ""
	}
	return "Bearer"
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

func (s Scenario) TurnsValue() []ConversationTurn {
	if len(s.Turns) > 0 {
		out := make([]ConversationTurn, 0, len(s.Turns))
		for _, turn := range s.Turns {
			role := strings.ToLower(strings.TrimSpace(turn.Role))
			content := strings.TrimSpace(turn.Content)
			if role == "" || !turnHasContent(turn, content) {
				continue
			}
			normalized := ConversationTurn{
				Role:       role,
				Content:    content,
				Images:     normalizeMediaInputs(turn.Images),
				Audio:      normalizeMediaInputs(turn.Audio),
				PDFs:       normalizeMediaInputs(turn.PDFs),
				Name:       strings.TrimSpace(turn.Name),
				ToolCallID: strings.TrimSpace(turn.ToolCallID),
			}
			out = append(out, normalized)
			out = append(out, expandMockToolResults(turn.MockToolResults)...)
		}
		return out
	}

	out := make([]ConversationTurn, 0, 2)
	if sys := strings.TrimSpace(s.System); sys != "" {
		out = append(out, ConversationTurn{Role: "system", Content: sys})
	}
	if input := strings.TrimSpace(s.Input); input != "" || len(s.Images) > 0 || len(s.Audio) > 0 || len(s.PDFs) > 0 {
		out = append(out, ConversationTurn{
			Role:    "user",
			Content: input,
			Images:  normalizeMediaInputs(s.Images),
			Audio:   normalizeMediaInputs(s.Audio),
			PDFs:    normalizeMediaInputs(s.PDFs),
		})
	}
	return out
}

func (s Scenario) ImagesValue() []MediaInput {
	return normalizeMediaInputs(s.Images)
}

func (s Scenario) AudioValue() []MediaInput {
	return normalizeMediaInputs(s.Audio)
}

func (s Scenario) PDFsValue() []MediaInput {
	return normalizeMediaInputs(s.PDFs)
}

func (s Scenario) JudgeOutputsValue() []JudgeOutput {
	return normalizeJudgeOutputs(s.JudgeOutputs)
}

func (s Scenario) SystemValue() string {
	if len(s.Turns) == 0 {
		return strings.TrimSpace(s.System)
	}
	parts := make([]string, 0, len(s.Turns))
	for _, turn := range s.TurnsValue() {
		if turn.Role == "system" {
			parts = append(parts, turn.Content)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (s Scenario) InputValue() string {
	if len(s.Turns) == 0 {
		if input := strings.TrimSpace(s.Input); input != "" {
			return input
		}
		return renderTurnText(ConversationTurn{
			Role:   "user",
			Images: s.ImagesValue(),
			Audio:  s.AudioValue(),
			PDFs:   s.PDFsValue(),
		})
	}
	for i := len(s.Turns) - 1; i >= 0; i-- {
		turn := s.Turns[i]
		if strings.EqualFold(strings.TrimSpace(turn.Role), "user") {
			return renderTurnText(turn)
		}
	}
	return ""
}

func (s Scenario) TranscriptText() string {
	turns := s.TurnsValue()
	if len(turns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(turns))
	for _, turn := range turns {
		label := turn.Role
		if turn.Name != "" {
			label = fmt.Sprintf("%s:%s", label, turn.Name)
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, renderTurnText(turn)))
	}
	return strings.Join(lines, "\n")
}

func turnHasContent(turn ConversationTurn, content string) bool {
	return content != "" || len(turn.Images) > 0 || len(turn.Audio) > 0 || len(turn.PDFs) > 0 || len(turn.MockToolResults) > 0
}

func normalizeMediaInputs(items []MediaInput) []MediaInput {
	if len(items) == 0 {
		return nil
	}
	out := make([]MediaInput, 0, len(items))
	for _, item := range items {
		if normalized, ok := normalizeMediaInput(item); ok {
			out = append(out, normalized)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeMediaInput(item MediaInput) (MediaInput, bool) {
	normalized := MediaInput{
		URL:       strings.TrimSpace(item.URL),
		Path:      strings.TrimSpace(item.Path),
		Data:      strings.TrimSpace(item.Data),
		MediaType: strings.TrimSpace(item.MediaType),
		Detail:    strings.TrimSpace(item.Detail),
		Filename:  strings.TrimSpace(item.Filename),
		Caption:   strings.TrimSpace(item.Caption),
	}
	if normalized.URL == "" && normalized.Path == "" && normalized.Data == "" {
		return MediaInput{}, false
	}
	return normalized, true
}

func normalizeJudgeOutputs(items []JudgeOutput) []JudgeOutput {
	if len(items) == 0 {
		return nil
	}
	out := make([]JudgeOutput, 0, len(items))
	for _, item := range items {
		normalized := JudgeOutput{
			Name:      strings.TrimSpace(item.Name),
			Type:      strings.ToLower(strings.TrimSpace(item.Type)),
			Path:      strings.TrimSpace(item.Path),
			Value:     strings.TrimSpace(item.Value),
			MediaType: strings.TrimSpace(item.MediaType),
		}
		if normalized.Type == "" && normalized.Path == "" && normalized.Value == "" && normalized.Name == "" && normalized.MediaType == "" {
			continue
		}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func renderTurnText(turn ConversationTurn) string {
	parts := make([]string, 0, 1+len(turn.Images)+len(turn.Audio)+len(turn.PDFs))
	if content := strings.TrimSpace(turn.Content); content != "" {
		parts = append(parts, content)
	}
	parts = append(parts, renderMediaSummary("image", turn.Images)...)
	parts = append(parts, renderMediaSummary("audio", turn.Audio)...)
	parts = append(parts, renderMediaSummary("pdf", turn.PDFs)...)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func expandMockToolResults(items []MockToolResult) []ConversationTurn {
	if len(items) == 0 {
		return nil
	}
	out := make([]ConversationTurn, 0, len(items)*2)
	for i, item := range items {
		normalized, ok := normalizeMockToolResult(item, i)
		if !ok {
			continue
		}
		out = append(out, ConversationTurn{
			Role:    "assistant",
			Content: renderMockToolCall(normalized),
		})
		out = append(out, ConversationTurn{
			Role:       "tool",
			Name:       normalized.Name,
			ToolCallID: normalized.ToolCallID,
			Content:    normalized.Content,
			Images:     normalized.Images,
			Audio:      normalized.Audio,
			PDFs:       normalized.PDFs,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeMockToolResult(item MockToolResult, index int) (MockToolResult, bool) {
	normalized := MockToolResult{
		Name:       strings.TrimSpace(item.Name),
		Arguments:  strings.TrimSpace(item.Arguments),
		Content:    strings.TrimSpace(item.Content),
		Images:     normalizeMediaInputs(item.Images),
		Audio:      normalizeMediaInputs(item.Audio),
		PDFs:       normalizeMediaInputs(item.PDFs),
		ToolCallID: strings.TrimSpace(item.ToolCallID),
	}
	if normalized.Name == "" {
		return MockToolResult{}, false
	}
	if normalized.ToolCallID == "" {
		normalized.ToolCallID = fmt.Sprintf("mock_tool_call_%d", index+1)
	}
	if normalized.Content == "" && len(normalized.Images) == 0 && len(normalized.Audio) == 0 && len(normalized.PDFs) == 0 {
		return MockToolResult{}, false
	}
	return normalized, true
}

func renderMockToolCall(item MockToolResult) string {
	if item.Arguments == "" {
		return fmt.Sprintf("[mock tool call] %s", item.Name)
	}
	return fmt.Sprintf("[mock tool call] %s %s", item.Name, item.Arguments)
}

func renderMediaSummary(kind string, items []MediaInput) []string {
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		label := mediaInputLabel(item)
		if caption := strings.TrimSpace(item.Caption); caption != "" {
			lines = append(lines, fmt.Sprintf("[%s] %s (%s)", kind, label, caption))
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", kind, label))
	}
	return lines
}

func mediaInputLabel(item MediaInput) string {
	switch {
	case strings.TrimSpace(item.Filename) != "":
		return strings.TrimSpace(item.Filename)
	case strings.TrimSpace(item.Path) != "":
		return strings.TrimSpace(item.Path)
	case strings.TrimSpace(item.URL) != "":
		return strings.TrimSpace(item.URL)
	case strings.TrimSpace(item.MediaType) != "":
		return strings.TrimSpace(item.MediaType)
	default:
		return "embedded"
	}
}
