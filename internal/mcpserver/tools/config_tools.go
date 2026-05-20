package tools

import (
	"context"
	"fmt"
	"strings"

	"cleanr/cleanr"
)

func callExampleConfig(_ context.Context, args map[string]any) (Result, error) {
	var input exampleConfigArgs
	if err := decodeArgs(args, &input); err != nil {
		return Result{}, err
	}

	format := normalizeConfigFormat(input.Format)
	data, err := cleanr.MarshalConfig(cleanr.ExampleConfig(), format)
	if err != nil {
		return Result{}, err
	}

	out := exampleConfigOutput{
		Format: format,
		Config: string(data),
	}
	return structuredToolResult(out, out.Config), nil
}

func callValidateConfig(_ context.Context, args map[string]any) (Result, error) {
	var input configSource
	if err := decodeArgs(args, &input); err != nil {
		return Result{}, err
	}

	cfg, err := loadConfigSource(input)
	if err != nil {
		out := validateConfigOutput{
			Valid:  false,
			Errors: []string{err.Error()},
		}
		return structuredToolResult(out, strings.Join(out.Errors, "\n")), nil
	}

	out := validateConfigOutput{
		Valid:         true,
		TargetName:    cfg.Target.Name,
		ScenarioCount: len(cfg.Scenarios),
	}
	return structuredToolResult(out, fmt.Sprintf("valid config for %s with %d scenarios", out.TargetName, out.ScenarioCount)), nil
}
