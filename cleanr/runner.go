package cleanr

import (
	"context"
	"net/http"
	"time"

	adapterspkg "cleanr/cleanr/adapters"
	enginespkg "cleanr/cleanr/engines"
	pluginspkg "cleanr/cleanr/plugins"
)

type Runner struct {
	config  Config
	target  Target
	engines []Engine
}

func NewRunner(cfg Config, target Target) *Runner {
	engines := defaultEngines(cfg)
	target = pluginspkg.WrapTarget(target, cfg.ResolvedPlugins)
	return &Runner{
		config:  cfg,
		target:  target,
		engines: engines,
	}
}

func NewConfigRunner(cfg Config) *Runner {
	client := &http.Client{Timeout: cfg.Target.Timeout()}
	return NewRunner(cfg, adapterspkg.NewTargetFromConfig(cfg.Target, client))
}

func NewHTTPRunner(cfg Config) *Runner {
	return NewConfigRunner(cfg)
}

func (r *Runner) Run(ctx context.Context) Report {
	start := time.Now()
	runCtx := &RunContext{Config: r.config, Target: r.target}
	report := Report{
		Name:        r.config.Target.Name,
		GeneratedAt: start.UTC(),
		Metadata:    buildRunMetadata(r.config),
	}

	for _, engine := range r.engines {
		suiteStart := time.Now()
		result := engine.Run(ctx, runCtx)
		result.Duration = time.Since(suiteStart)
		report.Suites = append(report.Suites, result)
		report.TotalSuites++
		if !result.Passed {
			report.FailedSuites++
		}
		for _, c := range result.Cases {
			report.TotalCases++
			if !c.Passed {
				report.FailedCases++
			}
		}
	}

	report.Passed = report.FailedSuites == 0 && report.FailedCases == 0
	report.Duration = time.Since(start)
	report.Recommendations = buildRecommendations(report)
	return report
}

func defaultEngines(cfg Config) []Engine {
	return enginespkg.Default(cfg)
}

func buildRecommendations(report Report) []string {
	var recs []string
	for _, suite := range report.Suites {
		if suite.Passed {
			continue
		}
		switch suite.Name {
		case "prompt-injection":
			recs = append(recs, "Harden system prompt boundaries and add explicit refusal templates for adversarial inputs.")
		case "security":
			recs = append(recs, "Add output sanitization for secrets, PII, and dangerous tool instructions before responses leave the app.")
		case "load":
			recs = append(recs, "Reduce latency variance under concurrency by adding rate limits, caching, and upstream timeout budgets.")
		case "chaos":
			recs = append(recs, "Increase resilience under degraded request conditions by validating inputs and handling duplicate or truncated context safely.")
		case "drift":
			recs = append(recs, "Stabilize prompts or decoding settings for deterministic paths and snapshot important regression scenarios.")
		case "shadow-state":
			recs = append(recs, "Restrict agent side effects to approved paths and verify the observed file mutations against an explicit allowlist.")
		case "provenance":
			recs = append(recs, "Separate trusted and untrusted context sources explicitly and require refusal or validation before untrusted context can influence privileged actions.")
		case "claim-trace":
			recs = append(recs, "Require the agent to ground tool, citation, approval, and mutation claims in normalized trace evidence before treating the run as releasable.")
		case "release-policy":
			recs = append(recs, "Encode action-level release rules for tools, approvals, trust boundaries, sinks, and state changes so CI can block unsafe workflows before merge.")
		case "memory-safety":
			recs = append(recs, "Partition memory by user and freshness, and block stale, revoked, poisoned, or cross-user memory from flowing into responses or tool calls.")
		case "token-optimization":
			recs = append(recs, "Reduce prompt and response token waste with tighter context windows, deduplicated instructions, and explicit output length caps.")
		}
	}
	return recs
}
