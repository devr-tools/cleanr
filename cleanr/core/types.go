package core

import (
	"context"
	"strings"
	"time"
)

type Config struct {
	Version   string          `json:"version"`
	Target    TargetConfig    `json:"target"`
	Scenarios []Scenario      `json:"scenarios"`
	Suites    SuitesConfig    `json:"suites"`
	Reporting ReportingConfig `json:"reporting"`
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

type ReportingConfig struct {
	Format     string          `json:"format"`
	Output     string          `json:"output"`
	TrendFile  string          `json:"trend_file"`
	TrendLimit int             `json:"trend_limit"`
	BuildID    string          `json:"build_id"`
	TrendGates TrendGateConfig `json:"trend_gates"`
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

type Request struct {
	Scenario Scenario
	System   string
	Prompt   string
	Timeout  time.Duration
	Headers  map[string]string
	Template any
}

type Response struct {
	StatusCode   int
	Body         []byte
	Text         string
	Latency      time.Duration
	Err          error
	ExtractError error
	Usage        TokenUsage
	Normalized   ProviderResponse
}

type TokenUsage struct {
	InputTokens  int  `json:"input_tokens,omitempty"`
	OutputTokens int  `json:"output_tokens,omitempty"`
	TotalTokens  int  `json:"total_tokens,omitempty"`
	Heuristic    bool `json:"heuristic,omitempty"`
}

type ProviderResponse struct {
	Provider         string             `json:"provider,omitempty"`
	ID               string             `json:"id,omitempty"`
	Model            string             `json:"model,omitempty"`
	Role             string             `json:"role,omitempty"`
	Status           string             `json:"status,omitempty"`
	FinishReason     string             `json:"finish_reason,omitempty"`
	StopSequence     string             `json:"stop_sequence,omitempty"`
	ToolCalls        []ToolCall         `json:"tool_calls,omitempty"`
	SourceUses       []SourceUse        `json:"source_uses,omitempty"`
	Approvals        []ApprovalArtifact `json:"approvals,omitempty"`
	StateChanges     []StateChange      `json:"state_changes,omitempty"`
	MemoryOperations []MemoryOperation  `json:"memory_operations,omitempty"`
	Raw              map[string]any     `json:"raw,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id,omitempty"`
	CallID    string         `json:"call_id,omitempty"`
	Type      string         `json:"type,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments string         `json:"arguments,omitempty"`
	Input     any            `json:"input,omitempty"`
	Status    string         `json:"status,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type SourceUse struct {
	ID       string         `json:"id,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Name     string         `json:"name,omitempty"`
	Location string         `json:"location,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type ApprovalArtifact struct {
	ID       string         `json:"id,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Status   string         `json:"status,omitempty"`
	Actor    string         `json:"actor,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Artifact string         `json:"artifact,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type StateChange struct {
	Kind    string         `json:"kind,omitempty"`
	Target  string         `json:"target,omitempty"`
	Action  string         `json:"action,omitempty"`
	Status  string         `json:"status,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Raw     map[string]any `json:"raw,omitempty"`
}

type MemoryOperation struct {
	Action    string         `json:"action,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	Key       string         `json:"key,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Value     string         `json:"value,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type Finding struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type CaseResult struct {
	Name       string         `json:"name"`
	Passed     bool           `json:"passed"`
	Duration   time.Duration  `json:"duration"`
	Score      float64        `json:"score,omitempty"`
	LatencyP95 time.Duration  `json:"latency_p95,omitempty"`
	Findings   []Finding      `json:"findings,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

type SuiteResult struct {
	Name     string         `json:"name"`
	Passed   bool           `json:"passed"`
	Duration time.Duration  `json:"duration"`
	Cases    []CaseResult   `json:"cases"`
	Findings []Finding      `json:"findings,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type Report struct {
	Name            string           `json:"name"`
	Passed          bool             `json:"passed"`
	GeneratedAt     time.Time        `json:"generated_at"`
	Duration        time.Duration    `json:"duration"`
	TotalSuites     int              `json:"total_suites"`
	FailedSuites    int              `json:"failed_suites"`
	TotalCases      int              `json:"total_cases"`
	FailedCases     int              `json:"failed_cases"`
	Suites          []SuiteResult    `json:"suites"`
	Trend           *TrendReport     `json:"trend,omitempty"`
	TrendGate       *TrendGateReport `json:"trend_gate,omitempty"`
	Recommendations []string         `json:"recommendations,omitempty"`
}

type TrendReport struct {
	Baseline         bool          `json:"baseline"`
	HistoryLength    int           `json:"history_length"`
	CurrentBuildID   string        `json:"current_build_id,omitempty"`
	PreviousBuildID  string        `json:"previous_build_id,omitempty"`
	PreviousAt       time.Time     `json:"previous_at,omitempty"`
	PreviousDuration time.Duration `json:"previous_duration,omitempty"`
	Summary          TrendSummary  `json:"summary"`
	Suites           []SuiteTrend  `json:"suites,omitempty"`
	CaseRegressions  []CaseTrend   `json:"case_regressions,omitempty"`
	CaseImprovements []CaseTrend   `json:"case_improvements,omitempty"`
	FailureBuckets   []FailureBucket `json:"failure_buckets,omitempty"`
}

type TrendGateReport struct {
	Enabled         bool      `json:"enabled"`
	Evaluated       bool      `json:"evaluated"`
	Passed          bool      `json:"passed"`
	RequiredWindow  int       `json:"required_window,omitempty"`
	AvailableWindow int       `json:"available_window,omitempty"`
	GeneratedAt     time.Time `json:"generated_at,omitempty"`
	Findings        []Finding `json:"findings,omitempty"`
}

type TrendSummary struct {
	FailedSuitesDelta int           `json:"failed_suites_delta"`
	FailedCasesDelta  int           `json:"failed_cases_delta"`
	DurationDelta     time.Duration `json:"duration_delta"`
	RegressedSuites   int           `json:"regressed_suites"`
	ImprovedSuites    int           `json:"improved_suites"`
}

type SuiteTrend struct {
	Name             string      `json:"name"`
	Status           string      `json:"status"`
	FailedCasesDelta int         `json:"failed_cases_delta"`
	ScoreDelta       float64     `json:"score_delta,omitempty"`
	Drift            *DriftTrend `json:"drift,omitempty"`
}

type CaseTrend struct {
	Suite                   string   `json:"suite"`
	Name                    string   `json:"name"`
	Status                  string   `json:"status"`
	Passed                  bool     `json:"passed"`
	FindingSignatures       []string `json:"finding_signatures,omitempty"`
	NewFindingSignatures    []string `json:"new_finding_signatures,omitempty"`
	ClearedFindingSignatures []string `json:"cleared_finding_signatures,omitempty"`
	FirstUnsupportedClaim   string   `json:"first_unsupported_claim,omitempty"`
	ToolCalls               []string `json:"tool_calls,omitempty"`
	StateChanges            []string `json:"state_changes,omitempty"`
	FileChanges             []string `json:"file_changes,omitempty"`
	MemoryMarkers           []string `json:"memory_markers,omitempty"`
}

type FailureBucket struct {
	Signature string   `json:"signature"`
	Count     int      `json:"count"`
	Cases     []string `json:"cases,omitempty"`
}

type DriftTrend struct {
	NormalizedDriftDelta          float64 `json:"normalized_drift_delta,omitempty"`
	SemanticDriftDelta            float64 `json:"semantic_drift_delta,omitempty"`
	ConsistencyScoreDelta         float64 `json:"consistency_score_delta,omitempty"`
	SemanticConsistencyScoreDelta float64 `json:"semantic_consistency_score_delta,omitempty"`
	BaselineDriftDelta            float64 `json:"baseline_drift_delta,omitempty"`
	BaselineSemanticDriftDelta    float64 `json:"baseline_semantic_drift_delta,omitempty"`
}

type Target interface {
	Invoke(context.Context, Request) Response
}

type Engine interface {
	Name() string
	Run(context.Context, *RunContext) SuiteResult
}

type RunContext struct {
	Config Config
	Target Target
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
