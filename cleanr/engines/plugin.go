package engines

import (
	"context"
	"fmt"

	"cleanr/cleanr/core"
	pluginspkg "cleanr/cleanr/plugins"
)

type PluginSuiteEngine struct {
	Manifest core.PluginManifest
	Suite    core.PluginSuite
}

func (e PluginSuiteEngine) Name() string {
	return e.Suite.Name
}

func (e PluginSuiteEngine) Run(ctx context.Context, runCtx *core.RunContext) core.SuiteResult {
	result, err := pluginspkg.RunPluginSuite(ctx, e.Manifest, e.Suite, runCtx.Config)
	if err != nil {
		return core.SuiteResult{
			Name:   e.Suite.Name,
			Passed: false,
			Findings: []core.Finding{{
				Severity: "critical",
				Message:  fmt.Sprintf("plugin suite %s failed: %v", e.Suite.Name, err),
			}},
		}
	}
	return result
}
