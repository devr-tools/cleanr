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
	Name              string            `json:"name"`
	System            string            `json:"system"`
	Input             string            `json:"input"`
	Metadata          map[string]string `json:"metadata"`
	Tags              []string          `json:"tags"`
	ExpectedContains  []string          `json:"expected_contains"`
	ForbiddenContains []string          `json:"forbidden_contains"`
	Assertions        []Assertion       `json:"assertions"`
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
	Format string `json:"format"`
	Output string `json:"output"`
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
	Provider     string         `json:"provider,omitempty"`
	ID           string         `json:"id,omitempty"`
	Model        string         `json:"model,omitempty"`
	Role         string         `json:"role,omitempty"`
	Status       string         `json:"status,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	ToolCalls    []ToolCall     `json:"tool_calls,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
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
	Name            string        `json:"name"`
	Passed          bool          `json:"passed"`
	GeneratedAt     time.Time     `json:"generated_at"`
	Duration        time.Duration `json:"duration"`
	TotalSuites     int           `json:"total_suites"`
	FailedSuites    int           `json:"failed_suites"`
	TotalCases      int           `json:"total_cases"`
	FailedCases     int           `json:"failed_cases"`
	Suites          []SuiteResult `json:"suites"`
	Recommendations []string      `json:"recommendations,omitempty"`
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
