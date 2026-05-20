package cleanr

import (
	"context"
	"fmt"
	"net/http"
	"time"

	enginespkg "cleanr/cleanr/engines"
)

type Runner struct {
	config  Config
	target  Target
	engines []Engine
}

func NewRunner(cfg Config, target Target) *Runner {
	engines := defaultEngines(cfg)
	return &Runner{
		config:  cfg,
		target:  target,
		engines: engines,
	}
}

func NewConfigRunner(cfg Config) *Runner {
	client := &http.Client{Timeout: cfg.Target.Timeout()}
	return NewRunner(cfg, newTargetFromConfig(cfg.Target, client))
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
		case "token-optimization":
			recs = append(recs, "Reduce prompt and response token waste with tighter context windows, deduplicated instructions, and explicit output length caps.")
		}
	}
	return recs
}

func newTargetFromConfig(cfg TargetConfig, client *http.Client) Target {
	switch cfg.TargetType() {
	case "openai":
		return NewOpenAITarget(cfg, client)
	case "http":
		return NewHTTPTarget(cfg, client)
	default:
		return invalidTarget{err: fmt.Errorf("unsupported target type %q", cfg.Type)}
	}
}

type invalidTarget struct {
	err error
}

func (t invalidTarget) Invoke(context.Context, Request) Response {
	return Response{Err: t.err}
}
