package runtime

import (
	"context"
	"fmt"
	"strings"

	"cleanr/cleanr"
	"cleanr/internal/mcpserver/toolkit"
)

func ExampleConfigDefinition() toolkit.Definition {
	return toolkit.Definition{
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
	}
}

func ValidateConfigDefinition() toolkit.Definition {
	return toolkit.Definition{
		Name:        "cleanr_validate_config",
		Title:       "Validate cleanr config",
		Description: "Validate a cleanr config provided inline or by local path.",
		InputSchema: toolkit.ConfigSourceSchema(),
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
	}
}

func ExampleConfig(_ context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.ExampleConfigArgs
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	format := toolkit.NormalizeConfigFormat(input.Format)
	data, err := cleanr.MarshalConfig(cleanr.ExampleConfig(), format)
	if err != nil {
		return toolkit.Result{}, err
	}

	out := toolkit.ExampleConfigOutput{
		Format: format,
		Config: string(data),
	}
	return toolkit.StructuredToolResult(out, out.Config), nil
}

func ValidateConfig(_ context.Context, args map[string]any) (toolkit.Result, error) {
	var input toolkit.ConfigSource
	if err := toolkit.DecodeArgs(args, &input); err != nil {
		return toolkit.Result{}, err
	}

	cfg, err := toolkit.LoadConfigSource(input)
	if err != nil {
		out := toolkit.ValidateConfigOutput{
			Valid:  false,
			Errors: []string{err.Error()},
		}
		return toolkit.StructuredToolResult(out, strings.Join(out.Errors, "\n")), nil
	}

	out := toolkit.ValidateConfigOutput{
		Valid:         true,
		TargetName:    cfg.Target.Name,
		ScenarioCount: len(cfg.Scenarios),
	}
	return toolkit.StructuredToolResult(out, fmt.Sprintf("valid config for %s with %d scenarios", out.TargetName, out.ScenarioCount)), nil
}
