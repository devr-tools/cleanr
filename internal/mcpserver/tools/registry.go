package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/devr-tools/cleanr/internal/mcpserver/catalog"
	"github.com/devr-tools/cleanr/internal/mcpserver/runtime"
	"github.com/devr-tools/cleanr/internal/mcpserver/toolkit"
)

type Definition = toolkit.Definition
type Result = toolkit.Result
type Content = toolkit.Content

// ErrUnknownTool marks a tools/call for a name that is not registered, so the
// server can answer with a protocol-level invalid-params error instead of a
// tool-execution failure.
var ErrUnknownTool = errors.New("unknown tool")

type handler func(context.Context, map[string]any) (toolkit.Result, error)

var definitions = []Definition{
	runtime.ExampleConfigDefinition(),
	runtime.ValidateConfigDefinition(),
	runtime.RunDefinition(),
	runtime.RenderReportDefinition(),
	runtime.GenerateDatasetDefinition(),
	runtime.ReviewDatasetDefinition(),
	runtime.AnalyzeTrendsDefinition(),
	runtime.ExplainFailuresDefinition(),
	catalog.SuiteDefinition(),
	catalog.TargetDefinition(),
}

var handlers = map[string]handler{
	"cleanr_example_config":    runtime.ExampleConfig,
	"cleanr_validate_config":   runtime.ValidateConfig,
	"cleanr_run":               runtime.Run,
	"cleanr_render_report":     runtime.RenderReport,
	"cleanr_generate_dataset":  runtime.GenerateDataset,
	"cleanr_review_dataset":    runtime.ReviewDataset,
	"cleanr_analyze_trends":    runtime.AnalyzeTrends,
	"cleanr_explain_failures":  runtime.ExplainFailures,
	"cleanr_describe_suites":   catalog.DescribeSuites,
	"cleanr_supported_targets": catalog.SupportedTargets,
}

func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

func Call(ctx context.Context, name string, args map[string]any) (Result, error) {
	h, ok := handlers[name]
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	return h(ctx, args)
}
