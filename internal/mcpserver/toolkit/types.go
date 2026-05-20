package toolkit

import "cleanr/cleanr"

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
