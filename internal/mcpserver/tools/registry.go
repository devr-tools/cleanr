package tools

import "context"

type handler func(context.Context, map[string]any) (Result, error)

var definitions = []Definition{
	{
		Name:        "cleanr_example_config",
		Title:       "Generate cleanr example config",
		Description: "Return a starter cleanr config in JSON or YAML for agent editing.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format": map[string]any{
					"type":        "string",
					"description": "Config format to generate.",
					"enum":        []string{"json", "yaml"},
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format": map[string]any{"type": "string"},
				"config": map[string]any{"type": "string"},
			},
			"required": []string{"format", "config"},
		},
	},
	{
		Name:        "cleanr_validate_config",
		Title:       "Validate cleanr config",
		Description: "Validate a cleanr config provided inline or by local path.",
		InputSchema: configSourceSchema(),
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"valid":          map[string]any{"type": "boolean"},
				"target_name":    map[string]any{"type": "string"},
				"scenario_count": map[string]any{"type": "integer"},
				"errors": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"required": []string{"valid"},
		},
	},
	{
		Name:        "cleanr_run",
		Title:       "Run cleanr suites",
		Description: "Execute cleanr against a config provided inline or by local path and return the report.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config": map[string]any{
					"type":        "string",
					"description": "Raw cleanr config content.",
				},
				"config_path": map[string]any{
					"type":        "string",
					"description": "Local path to a cleanr config file.",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Config format when config is provided inline.",
					"enum":        []string{"json", "yaml"},
				},
				"report_format": map[string]any{
					"type":        "string",
					"description": "Rendered report format.",
					"enum":        []string{"text", "json", "junit"},
				},
				"timeout_ms": map[string]any{
					"type":        "integer",
					"description": "Optional overall run timeout in milliseconds.",
					"minimum":     0,
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"passed":        map[string]any{"type": "boolean"},
				"exit_code":     map[string]any{"type": "integer"},
				"target_name":   map[string]any{"type": "string"},
				"report_format": map[string]any{"type": "string"},
				"report_text":   map[string]any{"type": "string"},
				"duration_ms":   map[string]any{"type": "integer"},
				"report":        map[string]any{"type": "object"},
				"error":         map[string]any{"type": "string"},
			},
			"required": []string{"passed", "exit_code", "report_format", "report_text"},
		},
	},
	{
		Name:        "cleanr_render_report",
		Title:       "Render cleanr report",
		Description: "Render a JSON cleanr report as text, json, or junit.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"report_json": map[string]any{
					"type":        "string",
					"description": "A serialized cleanr report JSON object.",
				},
				"format": map[string]any{
					"type":        "string",
					"description": "Render format.",
					"enum":        []string{"text", "json", "junit"},
				},
			},
			"required": []string{"report_json"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format":   map[string]any{"type": "string"},
				"rendered": map[string]any{"type": "string"},
			},
			"required": []string{"format", "rendered"},
		},
	},
	{
		Name:        "cleanr_describe_suites",
		Title:       "Describe cleanr suites",
		Description: "Return the built-in cleanr suites, what they check, and their key config fields.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"suites": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
			},
			"required": []string{"suites"},
		},
	},
	{
		Name:        "cleanr_supported_targets",
		Title:       "Describe cleanr targets",
		Description: "Return the supported cleanr target types and their key config fields.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"targets": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
			},
			"required": []string{"targets"},
		},
	},
}

var handlers = map[string]handler{
	"cleanr_example_config":    callExampleConfig,
	"cleanr_validate_config":   callValidateConfig,
	"cleanr_run":               callRun,
	"cleanr_render_report":     callRenderReport,
	"cleanr_describe_suites":   callDescribeSuites,
	"cleanr_supported_targets": callSupportedTargets,
}

func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

func Call(ctx context.Context, name string, args map[string]any) (Result, error) {
	h, ok := handlers[name]
	if !ok {
		return Result{}, errUnknownTool(name)
	}
	return h(ctx, args)
}
