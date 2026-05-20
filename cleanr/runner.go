package cleanr

import (
	"context"
	"net/http"
	"time"
)

type Target interface {
	Invoke(context.Context, Request) Response
}

type Runner struct {
	config  Config
	target  Target
	engines []Engine
}

type Engine interface {
	Name() string
	Run(context.Context, *RunContext) SuiteResult
}

type RunContext struct {
	Config Config
	Target Target
}

func NewRunner(cfg Config, target Target) *Runner {
	engines := defaultEngines(cfg)
	return &Runner{
		config:  cfg,
		target:  target,
		engines: engines,
	}
}

func NewHTTPRunner(cfg Config) *Runner {
	client := &http.Client{Timeout: cfg.Target.Timeout()}
	return NewRunner(cfg, NewHTTPTarget(cfg.Target, client))
}

func (r *Runner) Run(ctx context.Context) Report {
	start := time.Now()
	runCtx := &RunContext{Config: r.config, Target: r.target}
	report := Report{
		Name:        r.config.Target.Name,
		GeneratedAt: start.UTC(),
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
	var engines []Engine
	if cfg.Suites.PromptInjection.Enabled {
		engines = append(engines, PromptInjectionEngine{})
	}
	if cfg.Suites.Security.Enabled {
		engines = append(engines, SecurityEngine{})
	}
	if cfg.Suites.Load.Enabled {
		engines = append(engines, LoadEngine{})
	}
	if cfg.Suites.Chaos.Enabled {
		engines = append(engines, ChaosEngine{})
	}
	if cfg.Suites.Drift.Enabled {
		engines = append(engines, DriftEngine{})
	}
	return engines
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
		}
	}
	return recs
}
