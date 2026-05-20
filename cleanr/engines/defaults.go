package engines

import "cleanr/cleanr/core"

func Default(cfg core.Config) []core.Engine {
	var out []core.Engine
	if cfg.Suites.PromptInjection.Enabled {
		out = append(out, PromptInjectionEngine{})
	}
	if cfg.Suites.Security.Enabled {
		out = append(out, SecurityEngine{})
	}
	if cfg.Suites.Load.Enabled {
		out = append(out, LoadEngine{})
	}
	if cfg.Suites.Chaos.Enabled {
		out = append(out, ChaosEngine{})
	}
	if cfg.Suites.Drift.Enabled {
		out = append(out, DriftEngine{})
	}
	if cfg.Suites.TokenOptimization.Enabled {
		out = append(out, TokenOptimizationEngine{})
	}
	return out
}
