package engines

import "github.com/devr-tools/cleanr/cleanr/core"

func Default(cfg core.Config) []core.Engine {
	var out []core.Engine
	// Read-only engines that replay the same unmodified scenario request share a
	// per-run response cache so the target is invoked once per scenario instead
	// of once per engine.
	sharedResponses := newResponseCache()
	if cfg.Suites.PromptInjection.Enabled {
		out = append(out, PromptInjectionEngine{})
	}
	if cfg.Suites.Security.Enabled {
		out = append(out, SecurityEngine{cache: sharedResponses})
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
	if cfg.Suites.ShadowState.Enabled {
		out = append(out, ShadowStateEngine{})
	}
	if cfg.Suites.Provenance.Enabled {
		out = append(out, ProvenanceEngine{})
	}
	if cfg.Suites.ClaimTrace.Enabled {
		out = append(out, ClaimTraceEngine{})
	}
	if cfg.Suites.ReleasePolicy.Enabled {
		out = append(out, ReleasePolicyEngine{cache: sharedResponses})
	}
	if cfg.Suites.MemorySafety.Enabled {
		out = append(out, MemorySafetyEngine{})
	}
	if cfg.Suites.TokenOptimization.Enabled {
		out = append(out, TokenOptimizationEngine{cache: sharedResponses})
	}
	if cfg.Suites.LLMJudge.Enabled {
		out = append(out, LLMJudgeEngine{})
	}
	for _, manifest := range cfg.ResolvedPlugins {
		for _, suite := range manifest.Suites {
			out = append(out, PluginSuiteEngine{Manifest: manifest, Suite: suite})
		}
	}
	return out
}
