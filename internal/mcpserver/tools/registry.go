package tools

import (
	"context"
	"fmt"

	"cleanr/internal/mcpserver/catalog"
	"cleanr/internal/mcpserver/runtime"
	"cleanr/internal/mcpserver/toolkit"
)

type Definition = toolkit.Definition
type Result = toolkit.Result

type handler func(context.Context, map[string]any) (toolkit.Result, error)

var definitions = []Definition{
	runtime.ExampleConfigDefinition(),
	runtime.ValidateConfigDefinition(),
	runtime.RunDefinition(),
	runtime.RenderReportDefinition(),
	catalog.SuiteDefinition(),
	catalog.TargetDefinition(),
}

var handlers = map[string]handler{
	"cleanr_example_config":    runtime.ExampleConfig,
	"cleanr_validate_config":   runtime.ValidateConfig,
	"cleanr_run":               runtime.Run,
	"cleanr_render_report":     runtime.RenderReport,
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
		return Result{}, fmt.Errorf("unknown tool: %s", name)
	}
	return h(ctx, args)
}
