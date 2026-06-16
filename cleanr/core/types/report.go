package types

import "time"

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
	Name            string             `json:"name"`
	Passed          bool               `json:"passed"`
	GeneratedAt     time.Time          `json:"generated_at"`
	Duration        time.Duration      `json:"duration"`
	TotalSuites     int                `json:"total_suites"`
	FailedSuites    int                `json:"failed_suites"`
	TotalCases      int                `json:"total_cases"`
	FailedCases     int                `json:"failed_cases"`
	Metadata        *RunMetadata       `json:"metadata,omitempty"`
	Suites          []SuiteResult      `json:"suites"`
	Trend           *TrendReport       `json:"trend,omitempty"`
	TrendGate       *TrendGateReport   `json:"trend_gate,omitempty"`
	Integrations    *IntegrationReport `json:"integrations,omitempty"`
	Recommendations []string           `json:"recommendations,omitempty"`
}

type AgentOutputContract struct {
	Kind    string `json:"kind"`
	Format  string `json:"format"`
	Version string `json:"version"`
}

type AgentReport struct {
	Contract        AgentOutputContract  `json:"contract"`
	Summary         AgentReportSummary   `json:"summary"`
	Findings        []AgentFinding       `json:"findings,omitempty"`
	FixSuggestions  []AgentFixSuggestion `json:"fix_suggestions,omitempty"`
	Recommendations []string             `json:"recommendations,omitempty"`
	Report          Report               `json:"report"`
}

type AgentReportSummary struct {
	Target              string        `json:"target"`
	Passed              bool          `json:"passed"`
	GeneratedAt         time.Time     `json:"generated_at,omitempty"`
	Duration            time.Duration `json:"duration"`
	TotalSuites         int           `json:"total_suites"`
	FailedSuites        int           `json:"failed_suites"`
	TotalCases          int           `json:"total_cases"`
	FailedCases         int           `json:"failed_cases"`
	FindingCount        int           `json:"finding_count"`
	RecommendationCount int           `json:"recommendation_count"`
}

type AgentFinding struct {
	ID       string         `json:"id,omitempty"`
	Scope    string         `json:"scope"`
	Suite    string         `json:"suite,omitempty"`
	Case     string         `json:"case,omitempty"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

type AgentFixSuggestion struct {
	ID         string   `json:"id,omitempty"`
	Scope      string   `json:"scope"`
	Suite      string   `json:"suite,omitempty"`
	Case       string   `json:"case,omitempty"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Actions    []string `json:"actions,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
}

type IntegrationReport struct {
	LocalBlocking bool                    `json:"local_blocking"`
	RemoteMode    string                  `json:"remote_mode,omitempty"`
	TrendSources  []ExternalTrendReport   `json:"trend_sources,omitempty"`
	ResultSinks   []ResultSinkReport      `json:"result_sinks,omitempty"`
	Summaries     []SummaryArtifactReport `json:"summaries,omitempty"`
}

type ExternalTrendReport struct {
	Name            string        `json:"name"`
	SourceType      string        `json:"source_type,omitempty"`
	Blocking        bool          `json:"blocking"`
	BestEffort      bool          `json:"best_effort"`
	Status          string        `json:"status,omitempty"`
	Message         string        `json:"message,omitempty"`
	ViewURL         string        `json:"view_url,omitempty"`
	HistoryLength   int           `json:"history_length,omitempty"`
	LatestBuildID   string        `json:"latest_build_id,omitempty"`
	LatestAt        time.Time     `json:"latest_at,omitempty"`
	PreviousAt      time.Time     `json:"previous_at,omitempty"`
	ComparedBuildID string        `json:"compared_build_id,omitempty"`
	Summary         *TrendSummary `json:"summary,omitempty"`
	BuildDiff       *BuildDiff    `json:"build_diff,omitempty"`
}

type ResultSinkReport struct {
	Name       string `json:"name"`
	SinkType   string `json:"sink_type,omitempty"`
	Blocking   bool   `json:"blocking"`
	BestEffort bool   `json:"best_effort"`
	Published  bool   `json:"published"`
	Message    string `json:"message,omitempty"`
	RunURL     string `json:"run_url,omitempty"`
}

type SummaryArtifactReport struct {
	Name    string `json:"name"`
	Format  string `json:"format,omitempty"`
	Output  string `json:"output,omitempty"`
	Written bool   `json:"written"`
	Message string `json:"message,omitempty"`
}

type TrendReport struct {
	Baseline         bool            `json:"baseline"`
	HistoryLength    int             `json:"history_length"`
	CurrentBuildID   string          `json:"current_build_id,omitempty"`
	PreviousBuildID  string          `json:"previous_build_id,omitempty"`
	PreviousAt       time.Time       `json:"previous_at,omitempty"`
	PreviousDuration time.Duration   `json:"previous_duration,omitempty"`
	BuildDiff        *BuildDiff      `json:"build_diff,omitempty"`
	Summary          TrendSummary    `json:"summary"`
	Suites           []SuiteTrend    `json:"suites,omitempty"`
	CaseRegressions  []CaseTrend     `json:"case_regressions,omitempty"`
	CaseImprovements []CaseTrend     `json:"case_improvements,omitempty"`
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

type RunMetadata struct {
	BuildID              string                `json:"build_id,omitempty"`
	TargetType           string                `json:"target_type,omitempty"`
	ProviderModel        string                `json:"provider_model,omitempty"`
	ScenarioFingerprints []ScenarioFingerprint `json:"scenario_fingerprints,omitempty"`
}

type ScenarioFingerprint struct {
	Name              string   `json:"name"`
	SystemHash        string   `json:"system_hash,omitempty"`
	InputHash         string   `json:"input_hash,omitempty"`
	TurnsHash         string   `json:"turns_hash,omitempty"`
	TurnCount         int      `json:"turn_count,omitempty"`
	ContextHash       string   `json:"context_hash,omitempty"`
	MemoryReplayHash  string   `json:"memory_replay_hash,omitempty"`
	MemoryReplaySteps int      `json:"memory_replay_steps,omitempty"`
	TagsHash          string   `json:"tags_hash,omitempty"`
	Tags              []string `json:"tags,omitempty"`
}

type BuildDiff struct {
	TargetTypeBefore string         `json:"target_type_before,omitempty"`
	TargetTypeAfter  string         `json:"target_type_after,omitempty"`
	ModelBefore      string         `json:"model_before,omitempty"`
	ModelAfter       string         `json:"model_after,omitempty"`
	ScenarioChanges  []ScenarioDiff `json:"scenario_changes,omitempty"`
}

type ScenarioDiff struct {
	Name                string `json:"name"`
	Status              string `json:"status"`
	SystemChanged       bool   `json:"system_changed,omitempty"`
	InputChanged        bool   `json:"input_changed,omitempty"`
	TurnsChanged        bool   `json:"turns_changed,omitempty"`
	ContextChanged      bool   `json:"context_changed,omitempty"`
	MemoryReplayChanged bool   `json:"memory_replay_changed,omitempty"`
	TagsChanged         bool   `json:"tags_changed,omitempty"`
}

type SuiteTrend struct {
	Name             string      `json:"name"`
	Status           string      `json:"status"`
	FailedCasesDelta int         `json:"failed_cases_delta"`
	ScoreDelta       float64     `json:"score_delta,omitempty"`
	Drift            *DriftTrend `json:"drift,omitempty"`
}

type CaseTrend struct {
	Suite                    string   `json:"suite"`
	Name                     string   `json:"name"`
	Status                   string   `json:"status"`
	Passed                   bool     `json:"passed"`
	FindingSignatures        []string `json:"finding_signatures,omitempty"`
	NewFindingSignatures     []string `json:"new_finding_signatures,omitempty"`
	ClearedFindingSignatures []string `json:"cleared_finding_signatures,omitempty"`
	FirstUnsupportedClaim    string   `json:"first_unsupported_claim,omitempty"`
	ToolCalls                []string `json:"tool_calls,omitempty"`
	StateChanges             []string `json:"state_changes,omitempty"`
	FileChanges              []string `json:"file_changes,omitempty"`
	MemoryMarkers            []string `json:"memory_markers,omitempty"`
}

type FailureBucket struct {
	Signature string   `json:"signature"`
	Count     int      `json:"count"`
	Cases     []string `json:"cases,omitempty"`
}

type ReplayArtifact struct {
	Version      string               `json:"version"`
	Target       string               `json:"target"`
	BuildID      string               `json:"build_id,omitempty"`
	GeneratedAt  time.Time            `json:"generated_at"`
	Passed       bool                 `json:"passed"`
	FailedSuites int                  `json:"failed_suites"`
	FailedCases  int                  `json:"failed_cases"`
	Metadata     *RunMetadata         `json:"metadata,omitempty"`
	BuildDiff    *BuildDiff           `json:"build_diff,omitempty"`
	TrendSummary *TrendSummary        `json:"trend_summary,omitempty"`
	Failures     []ReplayArtifactCase `json:"failures,omitempty"`
}

type ReplayArtifactCase struct {
	Suite    string               `json:"suite"`
	Name     string               `json:"name"`
	Scenario *ScenarioFingerprint `json:"scenario,omitempty"`
	Findings []Finding            `json:"findings,omitempty"`
	Evidence map[string]any       `json:"evidence,omitempty"`
	Failed   bool                 `json:"failed"`
}

type ReleaseGateAttestation struct {
	Version     string               `json:"version"`
	Type        string               `json:"type"`
	GeneratedAt time.Time            `json:"generated_at"`
	Subject     AttestationSubject   `json:"subject"`
	Predicate   AttestationPredicate `json:"predicate"`
	Signature   AttestationSignature `json:"signature"`
}

type AttestationSubject struct {
	Target               string `json:"target"`
	BuildID              string `json:"build_id,omitempty"`
	ReportSHA256         string `json:"report_sha256"`
	ReplayArtifactSHA256 string `json:"replay_artifact_sha256,omitempty"`
}

type AttestationPredicate struct {
	Passed       bool          `json:"passed"`
	FailedSuites int           `json:"failed_suites"`
	FailedCases  int           `json:"failed_cases"`
	TrendSummary *TrendSummary `json:"trend_summary,omitempty"`
	Metadata     *RunMetadata  `json:"metadata,omitempty"`
}

type AttestationSignature struct {
	KeyID     string `json:"key_id,omitempty"`
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type DriftTrend struct {
	NormalizedDriftDelta          float64 `json:"normalized_drift_delta,omitempty"`
	SemanticDriftDelta            float64 `json:"semantic_drift_delta,omitempty"`
	ConsistencyScoreDelta         float64 `json:"consistency_score_delta,omitempty"`
	SemanticConsistencyScoreDelta float64 `json:"semantic_consistency_score_delta,omitempty"`
	BaselineDriftDelta            float64 `json:"baseline_drift_delta,omitempty"`
	BaselineSemanticDriftDelta    float64 `json:"baseline_semantic_drift_delta,omitempty"`
}
