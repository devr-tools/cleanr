package toolkit

import "github.com/devr-tools/cleanr/cleanr"

type Definition struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Result struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

type ConfigSource struct {
	Config     string `json:"config"`
	ConfigPath string `json:"config_path"`
	Format     string `json:"format"`
}

type ExampleConfigArgs struct {
	Format string `json:"format"`
}

type RenderReportArgs struct {
	ReportJSON string `json:"report_json"`
	Format     string `json:"format"`
}

type GenerateDatasetArgs struct {
	Config       string `json:"config"`
	ConfigPath   string `json:"config_path"`
	Format       string `json:"format"`
	OutputFormat string `json:"output_format"`
}

type ReviewDatasetArgs struct {
	Config            string                       `json:"config"`
	ConfigPath        string                       `json:"config_path"`
	Format            string                       `json:"format"`
	Dataset           string                       `json:"dataset"`
	DatasetPath       string                       `json:"dataset_path"`
	DatasetFormat     string                       `json:"dataset_format"`
	Policy            string                       `json:"policy"`
	PolicyPath        string                       `json:"policy_path"`
	PolicyFormat      string                       `json:"policy_format"`
	OutputFormat      string                       `json:"output_format"`
	Approve           []string                     `json:"approve"`
	Reject            []string                     `json:"reject"`
	PromoteStable     []string                     `json:"promote_stable"`
	PromoteRegression []string                     `json:"promote_regression"`
	AddTags           map[string][]string          `json:"add_tags"`
	SetTags           map[string][]string          `json:"set_tags"`
	SetMetadata       map[string]map[string]string `json:"set_metadata"`
}

type AnalyzeTrendsArgs struct {
	History       string `json:"history"`
	HistoryPath   string `json:"history_path"`
	HistoryFormat string `json:"history_format"`
	Window        int    `json:"window"`
	OutputFormat  string `json:"output_format"`
}

type ExplainFailuresArgs struct {
	Replay       string `json:"replay"`
	ReplayPath   string `json:"replay_path"`
	ReplayFormat string `json:"replay_format"`
	MaxCases     int    `json:"max_cases"`
}

type RunArgs struct {
	Config     string `json:"config"`
	ConfigPath string `json:"config_path"`
	Format     string `json:"format"`
	ReportType string `json:"report_format"`
	TimeoutMS  int    `json:"timeout_ms"`
}

type ExampleConfigOutput struct {
	Format string `json:"format"`
	Config string `json:"config"`
}

type ValidateConfigOutput struct {
	Valid         bool     `json:"valid"`
	TargetName    string   `json:"target_name,omitempty"`
	ScenarioCount int      `json:"scenario_count,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

type RunOutput struct {
	Passed       bool          `json:"passed"`
	ExitCode     int           `json:"exit_code"`
	TargetName   string        `json:"target_name,omitempty"`
	ReportFormat string        `json:"report_format"`
	ReportText   string        `json:"report_text"`
	DurationMS   int64         `json:"duration_ms,omitempty"`
	Report       cleanr.Report `json:"report,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type RenderReportOutput struct {
	Format   string `json:"format"`
	Rendered string `json:"rendered"`
}

type GenerateDatasetOutput struct {
	Format      string                 `json:"format"`
	DatasetText string                 `json:"dataset_text"`
	Dataset     cleanr.ScenarioDataset `json:"dataset"`
}

type ReviewDatasetOutput struct {
	Format              string                         `json:"format"`
	ReviewedDatasetText string                         `json:"reviewed_dataset_text"`
	ApprovedDatasetText string                         `json:"approved_dataset_text"`
	ReviewedDataset     cleanr.ReviewedScenarioDataset `json:"reviewed_dataset"`
	ApprovedDataset     cleanr.ScenarioDataset         `json:"approved_dataset"`
}

type AnalyzeTrendsOutput struct {
	Format   string               `json:"format"`
	Rendered string               `json:"rendered"`
	Analysis cleanr.TrendAnalysis `json:"analysis"`
}

type FailureExplanation struct {
	Suite              string   `json:"suite"`
	Case               string   `json:"case"`
	ScenarioName       string   `json:"scenario_name,omitempty"`
	Failed             bool     `json:"failed"`
	PrimaryReason      string   `json:"primary_reason"`
	Findings           []string `json:"findings,omitempty"`
	EvidenceHighlights []string `json:"evidence_highlights,omitempty"`
}

type ExplainFailuresOutput struct {
	Target       string                 `json:"target,omitempty"`
	BuildID      string                 `json:"build_id,omitempty"`
	FailureCount int                    `json:"failure_count"`
	BucketCount  int                    `json:"bucket_count"`
	Buckets      []cleanr.FailureBucket `json:"buckets,omitempty"`
	Explanations []FailureExplanation   `json:"explanations,omitempty"`
	Summary      string                 `json:"summary"`
}

type SuiteCatalogOutput struct {
	Suites []SuiteDescriptor `json:"suites"`
}

type SuiteDescriptor struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ConfigFields []string `json:"config_fields"`
}

type TargetCatalogOutput struct {
	Targets []TargetDescriptor `json:"targets"`
}

type TargetDescriptor struct {
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	ConfigFields []string `json:"config_fields"`
}
