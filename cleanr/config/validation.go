package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"cleanr/cleanr/core"
)

func ValidateConfig(cfg core.Config) error {
	var errs ValidationErrors

	switch cfg.Target.TargetType() {
	case "http":
		requireNonEmpty(&errs, "target.url", cfg.Target.URL, "set target.url to the full API endpoint URL")
		requireNonEmpty(&errs, "target.prompt_field", cfg.Target.PromptField, "set target.prompt_field to the request field that receives the prompt text")
		requireNonEmpty(&errs, "target.response_field", cfg.Target.ResponseField, "set target.response_field to the JSON path that contains the model text response")
		if rawURL := strings.TrimSpace(cfg.Target.URL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add("target.url", "must be an absolute http(s) URL", "use a value such as http://localhost:8080/v1/chat or https://api.example.com/v1/chat")
			}
		}
	case "openai":
		requireNonEmpty(&errs, "target.openai.model", cfg.Target.OpenAI.Model, "set the OpenAI model name, for example gpt-4o-mini or gpt-4.1-mini")
		switch cfg.Target.OpenAI.APIModeValue() {
		case "responses", "chat_completions":
		default:
			errs.Add("target.openai.api_mode", "must be one of responses or chat_completions", "use responses for new projects or chat_completions for legacy-compatible message requests")
		}
		if rawURL := strings.TrimSpace(cfg.Target.OpenAI.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add("target.openai.base_url", "must be an absolute http(s) URL", "use a value such as https://api.openai.com/v1 or a compatible base URL for testing")
			}
		}
	case "anthropic":
		requireNonEmpty(&errs, "target.anthropic.model", cfg.Target.Anthropic.Model, "set the Anthropic model name, for example claude-sonnet-4-20250514")
		if rawURL := strings.TrimSpace(cfg.Target.Anthropic.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add("target.anthropic.base_url", "must be an absolute http(s) URL", "use a value such as https://api.anthropic.com/v1 or a compatible base URL for testing")
			}
		}
		if cfg.Target.Anthropic.MaxTokens < 0 {
			errs.Add("target.anthropic.max_tokens", "must be >= 0", "set a positive max_tokens budget or omit the field to use the default")
		}
	default:
		errs.Add("target.type", "must be one of http, openai, or anthropic", "omit target.type for the default HTTP adapter, or set it to openai or anthropic for a native provider adapter")
	}

	if cfg.Target.TimeoutMS < 0 {
		errs.Add("target.timeout_ms", "must be >= 0", "remove the value to use the default timeout, or set a positive millisecond value")
	}

	scenarioNames := make(map[string]int, len(cfg.Scenarios))
	if len(cfg.Scenarios) == 0 {
		errs.Add("scenarios", "at least one scenario is required", "add a scenario with both name and input so cleanr has something to execute")
	}
	for i, scenario := range cfg.Scenarios {
		prefix := fmt.Sprintf("scenarios[%d]", i)
		requireNonEmpty(&errs, prefix+".name", scenario.Name, "set a short stable scenario name, for example \"happy-path\"")
		requireNonEmpty(&errs, prefix+".input", scenario.Input, "set the end-user prompt or test input for this scenario")
		if name := strings.TrimSpace(scenario.Name); name != "" {
			if first, ok := scenarioNames[name]; ok {
				errs.Add(prefix+".name", fmt.Sprintf("duplicates scenarios[%d].name", first), "rename duplicate scenarios so each scenario name is unique in reports")
			} else {
				scenarioNames[name] = i
			}
		}
	}

	if cfg.Suites.Load.Enabled {
		requirePositiveInt(&errs, "suites.load.virtual_users", cfg.Suites.Load.VirtualUsers, "set virtual_users to at least 1 when the load suite is enabled")
		requirePositiveInt(&errs, "suites.load.requests_per_user", cfg.Suites.Load.RequestsPerUser, "set requests_per_user to at least 1 when the load suite is enabled")
		if cfg.Suites.Load.MaxErrorRatePct < 0 || cfg.Suites.Load.MaxErrorRatePct > 100 {
			errs.Add("suites.load.max_error_rate_pct", "must be between 0 and 100", "use a whole-number percentage such as 5 or 25")
		}
		if cfg.Suites.Load.P95LatencyMS < 0 {
			errs.Add("suites.load.p95_latency_ms", "must be >= 0", "set a positive latency budget in milliseconds")
		}
	}

	for i, pattern := range cfg.Suites.Security.LeakPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			errs.Add(fmt.Sprintf("suites.security.leak_patterns[%d]", i), "must be a valid Go regular expression", "fix the pattern syntax or remove the entry if it is no longer needed")
		}
	}

	if cfg.Suites.Chaos.Enabled {
		allowedFaults := map[string]struct{}{
			"tight_deadline":   {},
			"context_overflow": {},
			"duplicate_turn":   {},
		}
		for i, fault := range cfg.Suites.Chaos.Faults {
			if _, ok := allowedFaults[fault]; !ok {
				errs.Add(fmt.Sprintf("suites.chaos.faults[%d]", i), "must be one of tight_deadline, context_overflow, or duplicate_turn", "replace the value with a supported built-in chaos fault")
			}
		}
		if cfg.Suites.Chaos.TimeoutScale <= 0 {
			errs.Add("suites.chaos.timeout_scale", "must be > 0", "use a fractional multiplier such as 0.4 to shorten the timeout")
		}
		if cfg.Suites.Chaos.NoiseBytes < 0 {
			errs.Add("suites.chaos.noise_bytes", "must be >= 0", "set a non-negative number of injected bytes")
		}
		if cfg.Suites.Chaos.MaxErrorRate < 0 || cfg.Suites.Chaos.MaxErrorRate > 100 {
			errs.Add("suites.chaos.max_error_rate_pct", "must be between 0 and 100", "use a whole-number percentage such as 35")
		}
	}

	if cfg.Suites.Drift.Enabled {
		if cfg.Suites.Drift.Iterations < 2 {
			errs.Add("suites.drift.iterations", "must be >= 2", "set iterations to 2 or more so drift can compare repeated runs")
		}
		if cfg.Suites.Drift.MaxNormalizedDrift < 0 || cfg.Suites.Drift.MaxNormalizedDrift > 1 {
			errs.Add("suites.drift.max_normalized_drift", "must be between 0 and 1", "use a decimal threshold such as 0.3")
		}
		if cfg.Suites.Drift.MinConsistencyScore < 0 || cfg.Suites.Drift.MinConsistencyScore > 1 {
			errs.Add("suites.drift.min_consistency_score", "must be between 0 and 1", "use a decimal threshold such as 0.7")
		}
	}

	if cfg.Suites.TokenOptimization.Enabled {
		if cfg.Suites.TokenOptimization.MaxInputTokens < 0 {
			errs.Add("suites.token_optimization.max_input_tokens", "must be >= 0", "set a non-negative token budget or omit the field to use the default")
		}
		if cfg.Suites.TokenOptimization.MaxOutputTokens < 0 {
			errs.Add("suites.token_optimization.max_output_tokens", "must be >= 0", "set a non-negative token budget or omit the field to use the default")
		}
		if cfg.Suites.TokenOptimization.MaxTotalTokens < 0 {
			errs.Add("suites.token_optimization.max_total_tokens", "must be >= 0", "set a non-negative token budget or omit the field to use the default")
		}
		if cfg.Suites.TokenOptimization.MaxOutputInputRatio <= 0 {
			errs.Add("suites.token_optimization.max_output_input_ratio", "must be > 0", "use a positive ratio such as 1.4")
		}
		if cfg.Suites.TokenOptimization.MaxPromptDuplicationRatio < 0 || cfg.Suites.TokenOptimization.MaxPromptDuplicationRatio > 1 {
			errs.Add("suites.token_optimization.max_prompt_duplication_ratio", "must be between 0 and 1", "use a decimal ratio such as 0.18")
		}
		if cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio < 0 || cfg.Suites.TokenOptimization.MaxResponseDuplicationRatio > 1 {
			errs.Add("suites.token_optimization.max_response_duplication_ratio", "must be between 0 and 1", "use a decimal ratio such as 0.12")
		}
		if cfg.Suites.TokenOptimization.SuggestedMaxOutputTokens < 0 {
			errs.Add("suites.token_optimization.suggested_max_output_tokens", "must be >= 0", "set a non-negative suggestion or omit the field to use the default")
		}
	}

	if format := strings.TrimSpace(cfg.Reporting.Format); format != "" {
		switch format {
		case "text", "json", "junit":
		default:
			errs.Add("reporting.format", "must be one of text, json, or junit", "use one of the built-in report formats or omit the field for text output")
		}
	}

	if errs.HasAny() {
		return errs
	}
	return nil
}

func requireNonEmpty(errs *ValidationErrors, path, value, hint string) {
	if strings.TrimSpace(value) == "" {
		errs.Add(path, "is required", hint)
	}
}

func requirePositiveInt(errs *ValidationErrors, path string, value int, hint string) {
	if value <= 0 {
		errs.Add(path, "must be > 0", hint)
	}
}
