package trends

import (
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type HistoryFile struct {
	Version   string       `json:"version"`
	Target    string       `json:"target"`
	UpdatedAt time.Time    `json:"updated_at"`
	Runs      []HistoryRun `json:"runs"`
}

type HistoryRun struct {
	BuildID      string            `json:"build_id,omitempty"`
	GeneratedAt  time.Time         `json:"generated_at"`
	Passed       bool              `json:"passed"`
	Duration     time.Duration     `json:"duration"`
	FailedSuites int               `json:"failed_suites"`
	FailedCases  int               `json:"failed_cases"`
	Metadata     *core.RunMetadata `json:"metadata,omitempty"`
	Suites       []HistorySuite    `json:"suites"`
}

type HistorySuite struct {
	Name         string               `json:"name"`
	Passed       bool                 `json:"passed"`
	FailedCases  int                  `json:"failed_cases"`
	AverageScore float64              `json:"average_score,omitempty"`
	Drift        *HistoryDriftMetrics `json:"drift,omitempty"`
	Load         *HistoryLoadMetrics  `json:"load,omitempty"`
	Cases        []HistoryCase        `json:"cases,omitempty"`
}

type HistoryCase struct {
	Name                  string   `json:"name"`
	Passed                bool     `json:"passed"`
	FindingSignatures     []string `json:"finding_signatures,omitempty"`
	FirstUnsupportedClaim string   `json:"first_unsupported_claim,omitempty"`
	ToolCalls             []string `json:"tool_calls,omitempty"`
	StateChanges          []string `json:"state_changes,omitempty"`
	FileChanges           []string `json:"file_changes,omitempty"`
	MemoryMarkers         []string `json:"memory_markers,omitempty"`
}

type HistoryDriftMetrics struct {
	Cases                    int     `json:"cases"`
	NormalizedDrift          float64 `json:"normalized_drift,omitempty"`
	SemanticDrift            float64 `json:"semantic_drift,omitempty"`
	ConsistencyScore         float64 `json:"consistency_score,omitempty"`
	SemanticConsistencyScore float64 `json:"semantic_consistency_score,omitempty"`
	BaselineDrift            float64 `json:"baseline_drift,omitempty"`
	BaselineSemanticDrift    float64 `json:"baseline_semantic_drift,omitempty"`
}

type HistoryLoadMetrics struct {
	Requests        int     `json:"requests,omitempty"`
	VirtualUsers    int     `json:"virtual_users,omitempty"`
	RequestsPerUser int     `json:"requests_per_user,omitempty"`
	ScenarioCount   int     `json:"scenario_count,omitempty"`
	ErrorRatePct    int     `json:"error_rate_pct,omitempty"`
	P50LatencyMS    int64   `json:"p50_latency_ms,omitempty"`
	P95LatencyMS    int64   `json:"p95_latency_ms,omitempty"`
	P99LatencyMS    int64   `json:"p99_latency_ms,omitempty"`
	ThroughputRPS   float64 `json:"throughput_rps,omitempty"`
}

type PersistOptions struct {
	Path       string
	BuildID    string
	TrendLimit int
}

type Analysis struct {
	Version           string               `json:"version"`
	Target            string               `json:"target"`
	TotalRetainedRuns int                  `json:"total_retained_runs"`
	WindowSize        int                  `json:"window_size"`
	PassRate          float64              `json:"pass_rate"`
	FailedRuns        int                  `json:"failed_runs"`
	AverageDuration   time.Duration        `json:"average_duration"`
	OldestAt          time.Time            `json:"oldest_at,omitempty"`
	Latest            RunSnapshot          `json:"latest"`
	Previous          *RunSnapshot         `json:"previous,omitempty"`
	Delta             *AnalysisDelta       `json:"delta,omitempty"`
	BuildDiff         *core.BuildDiff      `json:"build_diff,omitempty"`
	Regressions       []core.SuiteTrend    `json:"regressions,omitempty"`
	Improvements      []core.SuiteTrend    `json:"improvements,omitempty"`
	CaseRegressions   []core.CaseTrend     `json:"case_regressions,omitempty"`
	CaseImprovements  []core.CaseTrend     `json:"case_improvements,omitempty"`
	FailureBuckets    []core.FailureBucket `json:"failure_buckets,omitempty"`
	Drift             *DriftWindow         `json:"drift,omitempty"`
	Load              *LoadWindow          `json:"load,omitempty"`
	RecentRuns        []RunSnapshot        `json:"recent_runs,omitempty"`
}

type RunSnapshot struct {
	BuildID      string        `json:"build_id,omitempty"`
	GeneratedAt  time.Time     `json:"generated_at"`
	Passed       bool          `json:"passed"`
	FailedSuites int           `json:"failed_suites"`
	FailedCases  int           `json:"failed_cases"`
	Duration     time.Duration `json:"duration"`
}

type AnalysisDelta struct {
	FailedSuitesDelta int           `json:"failed_suites_delta"`
	FailedCasesDelta  int           `json:"failed_cases_delta"`
	DurationDelta     time.Duration `json:"duration_delta"`
	RegressedSuites   int           `json:"regressed_suites"`
	ImprovedSuites    int           `json:"improved_suites"`
}

type DriftWindow struct {
	AverageNormalizedDrift      float64 `json:"average_normalized_drift,omitempty"`
	AverageSemanticDrift        float64 `json:"average_semantic_drift,omitempty"`
	AverageConsistencyScore     float64 `json:"average_consistency_score,omitempty"`
	AverageSemanticConsistency  float64 `json:"average_semantic_consistency_score,omitempty"`
	MaxNormalizedDrift          float64 `json:"max_normalized_drift,omitempty"`
	MaxSemanticDrift            float64 `json:"max_semantic_drift,omitempty"`
	LatestNormalizedDrift       float64 `json:"latest_normalized_drift,omitempty"`
	LatestSemanticDrift         float64 `json:"latest_semantic_drift,omitempty"`
	LatestBaselineDrift         float64 `json:"latest_baseline_drift,omitempty"`
	LatestBaselineSemanticDrift float64 `json:"latest_baseline_semantic_drift,omitempty"`
}

type LoadWindow struct {
	Runs                 int     `json:"runs"`
	AverageErrorRatePct  float64 `json:"average_error_rate_pct,omitempty"`
	AverageP50LatencyMS  float64 `json:"average_p50_latency_ms,omitempty"`
	AverageP95LatencyMS  float64 `json:"average_p95_latency_ms,omitempty"`
	AverageP99LatencyMS  float64 `json:"average_p99_latency_ms,omitempty"`
	AverageThroughputRPS float64 `json:"average_throughput_rps,omitempty"`
	LatestErrorRatePct   int     `json:"latest_error_rate_pct,omitempty"`
	LatestP50LatencyMS   int64   `json:"latest_p50_latency_ms,omitempty"`
	LatestP95LatencyMS   int64   `json:"latest_p95_latency_ms,omitempty"`
	LatestP99LatencyMS   int64   `json:"latest_p99_latency_ms,omitempty"`
	LatestThroughputRPS  float64 `json:"latest_throughput_rps,omitempty"`
}

func NewHistory(target string) HistoryFile {
	return HistoryFile{
		Version: "v1alpha1",
		Target:  target,
		Runs:    []HistoryRun{},
	}
}

func AppendRun(history HistoryFile, run HistoryRun, limit int) HistoryFile {
	history.Runs = append(history.Runs, run)
	if limit > 0 && len(history.Runs) > limit {
		history.Runs = append([]HistoryRun(nil), history.Runs[len(history.Runs)-limit:]...)
	}
	history.UpdatedAt = time.Now().UTC()
	return history
}

func LatestRun(history HistoryFile) *HistoryRun {
	if len(history.Runs) == 0 {
		return nil
	}
	return &history.Runs[len(history.Runs)-1]
}

func BuildRun(report core.Report, buildID string) HistoryRun {
	run := HistoryRun{
		BuildID:      buildID,
		GeneratedAt:  report.GeneratedAt,
		Passed:       report.Passed,
		Duration:     report.Duration,
		FailedSuites: report.FailedSuites,
		FailedCases:  report.FailedCases,
		Metadata:     report.Metadata,
		Suites:       make([]HistorySuite, 0, len(report.Suites)),
	}
	for _, suite := range report.Suites {
		run.Suites = append(run.Suites, buildSuite(suite))
	}
	return run
}
