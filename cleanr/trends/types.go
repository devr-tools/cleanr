package trends

import (
	"time"

	"cleanr/cleanr/core"
)

type HistoryFile struct {
	Version   string       `json:"version"`
	Target    string       `json:"target"`
	UpdatedAt time.Time    `json:"updated_at"`
	Runs      []HistoryRun `json:"runs"`
}

type HistoryRun struct {
	BuildID      string         `json:"build_id,omitempty"`
	GeneratedAt  time.Time      `json:"generated_at"`
	Passed       bool           `json:"passed"`
	Duration     time.Duration  `json:"duration"`
	FailedSuites int            `json:"failed_suites"`
	FailedCases  int            `json:"failed_cases"`
	Suites       []HistorySuite `json:"suites"`
}

type HistorySuite struct {
	Name         string               `json:"name"`
	Passed       bool                 `json:"passed"`
	FailedCases  int                  `json:"failed_cases"`
	AverageScore float64              `json:"average_score,omitempty"`
	Drift        *HistoryDriftMetrics `json:"drift,omitempty"`
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

type PersistOptions struct {
	Path       string
	BuildID    string
	TrendLimit int
}

type Analysis struct {
	Version           string            `json:"version"`
	Target            string            `json:"target"`
	TotalRetainedRuns int               `json:"total_retained_runs"`
	WindowSize        int               `json:"window_size"`
	PassRate          float64           `json:"pass_rate"`
	FailedRuns        int               `json:"failed_runs"`
	AverageDuration   time.Duration     `json:"average_duration"`
	OldestAt          time.Time         `json:"oldest_at,omitempty"`
	Latest            RunSnapshot       `json:"latest"`
	Previous          *RunSnapshot      `json:"previous,omitempty"`
	Delta             *AnalysisDelta    `json:"delta,omitempty"`
	Regressions       []core.SuiteTrend `json:"regressions,omitempty"`
	Improvements      []core.SuiteTrend `json:"improvements,omitempty"`
	Drift             *DriftWindow      `json:"drift,omitempty"`
	RecentRuns        []RunSnapshot     `json:"recent_runs,omitempty"`
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
		Suites:       make([]HistorySuite, 0, len(report.Suites)),
	}
	for _, suite := range report.Suites {
		run.Suites = append(run.Suites, buildSuite(suite))
	}
	return run
}
