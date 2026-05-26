package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func validateTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	urlHint := "set the full API endpoint URL"
	promptHint := "set the request field that receives the prompt text"
	responseHint := "set the JSON path that contains the model text response"
	if prefix == "target" {
		urlHint = "set target.url to the full API endpoint URL"
		promptHint = "set target.prompt_field to the request field that receives the prompt text"
		responseHint = "set target.response_field to the JSON path that contains the model text response"
	}
	switch cfg.TargetType() {
	case "http":
		requireNonEmpty(errs, prefix+".url", cfg.URL, urlHint)
		requireNonEmpty(errs, prefix+".prompt_field", cfg.PromptField, promptHint)
		requireNonEmpty(errs, prefix+".response_field", cfg.ResponseField, responseHint)
		if rawURL := strings.TrimSpace(cfg.URL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".url", "must be an absolute http(s) URL", "use a value such as http://localhost:8080/v1/chat or https://api.example.com/v1/chat")
			}
		}
	case "openai":
		requireNonEmpty(errs, prefix+".openai.model", cfg.OpenAI.Model, "set the OpenAI model name, for example gpt-4o-mini or gpt-4.1-mini")
		switch cfg.OpenAI.APIModeValue() {
		case "responses", "chat_completions":
		default:
			errs.Add(prefix+".openai.api_mode", "must be one of responses or chat_completions", "use responses for new projects or chat_completions for legacy-compatible message requests")
		}
		if rawURL := strings.TrimSpace(cfg.OpenAI.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".openai.base_url", "must be an absolute http(s) URL", "use a value such as https://api.openai.com/v1 or a compatible base URL for testing")
			}
		}
	case "anthropic":
		requireNonEmpty(errs, prefix+".anthropic.model", cfg.Anthropic.Model, "set the Anthropic model name, for example claude-sonnet-4-20250514")
		if rawURL := strings.TrimSpace(cfg.Anthropic.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".anthropic.base_url", "must be an absolute http(s) URL", "use a value such as https://api.anthropic.com/v1 or a compatible base URL for testing")
			}
		}
		if cfg.Anthropic.MaxTokens < 0 {
			errs.Add(prefix+".anthropic.max_tokens", "must be >= 0", "set a positive max_tokens budget or omit the field to use the default")
		}
	default:
		errs.Add(prefix+".type", "must be one of http, openai, or anthropic", "set the target type to http, openai, or anthropic")
	}
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

func validateStringList(errs *ValidationErrors, path string, values []string, hint string) {
	for i, value := range values {
		requireNonEmpty(errs, fmt.Sprintf("%s[%d]", path, i), value, hint)
	}
}

func validateNonNegativeInt(errs *ValidationErrors, path string, value int, hint string) {
	if value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validatePositiveFloat(errs *ValidationErrors, path string, value float64, hint string) {
	if value <= 0 {
		errs.Add(path, "must be > 0", hint)
	}
}

func validateOptionalNonNegativeInt(errs *ValidationErrors, path string, value *int, hint string) {
	if value != nil && *value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validateNonNegativeFloat(errs *ValidationErrors, path string, value float64, hint string) {
	if value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validateOptionalNonNegativeFloat(errs *ValidationErrors, path string, value *float64, hint string) {
	if value != nil && *value < 0 {
		errs.Add(path, "must be >= 0", hint)
	}
}

func validateUnitInterval(errs *ValidationErrors, path string, value float64, hint string) {
	if value < 0 || value > 1 {
		errs.Add(path, "must be between 0 and 1", hint)
	}
}

func validateOptionalUnitInterval(errs *ValidationErrors, path string, value *float64, hint string) {
	if value != nil && (*value < 0 || *value > 1) {
		errs.Add(path, "must be between 0 and 1", hint)
	}
}

func validateAssertion(errs *ValidationErrors, prefix string, assertion core.Assertion) {
	switch strings.TrimSpace(assertion.Type) {
	case "contains", "not_contains":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the text fragment the response should include or exclude")
	case "regex":
		requireNonEmpty(errs, prefix+".pattern", assertion.Pattern, "set a valid Go regular expression to match against the response field")
		if strings.TrimSpace(assertion.Pattern) != "" {
			if _, err := regexp.Compile(assertion.Pattern); err != nil {
				errs.Add(prefix+".pattern", "must be a valid Go regular expression", "fix the pattern syntax or remove the assertion")
			}
		}
	case "json_path":
		requireNonEmpty(errs, prefix+".path", assertion.Path, "set the response path to check, for example response.provider_model or response.body.output.0.content.0.text")
	case "status_code":
		if assertion.IntValue == nil {
			errs.Add(prefix+".int_value", "is required", "set the expected HTTP status code such as 200")
		} else if *assertion.IntValue < 100 || *assertion.IntValue > 599 {
			errs.Add(prefix+".int_value", "must be between 100 and 599", "use a valid HTTP status code")
		}
	case "latency_ms":
		if assertion.IntValue == nil {
			errs.Add(prefix+".int_value", "is required", "set the maximum allowed latency in milliseconds")
		} else if *assertion.IntValue < 0 {
			errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative millisecond threshold")
		}
	case "finish_reason", "tool_call_name":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the expected provider finish reason or tool name")
	case "tool_call_count":
		if assertion.IntValue == nil {
			errs.Add(prefix+".int_value", "is required", "set the expected number of tool calls, including 0 when no tool calls should be present")
		} else if *assertion.IntValue < 0 {
			errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative expected tool call count")
		}
	default:
		errs.Add(prefix+".type", "must be one of contains, not_contains, regex, json_path, status_code, latency_ms, finish_reason, tool_call_count, or tool_call_name", "pick one of the built-in assertion types")
	}

	if severity := strings.TrimSpace(assertion.Severity); severity != "" {
		switch severity {
		case "low", "medium", "high", "critical":
		default:
			errs.Add(prefix+".severity", "must be one of low, medium, high, or critical", "omit severity to use the default, or pick a supported severity level")
		}
	}
}

func validateContextSource(errs *ValidationErrors, prefix string, source core.ContextSource) {
	requireNonEmpty(errs, prefix+".kind", source.Kind, "set the source type, for example retrieved, tool, memory, or approval")
	requireNonEmpty(errs, prefix+".trust", source.Trust, "set the trust tier, for example trusted, untrusted, or approved")
	requireNonEmpty(errs, prefix+".content", source.Content, "set the source content that should be provided to the model during testing")

	switch strings.TrimSpace(source.Kind) {
	case "retrieved", "tool", "memory", "approval":
	case "":
	default:
		errs.Add(prefix+".kind", "must be one of retrieved, tool, memory, or approval", "pick a supported context source kind")
	}

	switch strings.TrimSpace(source.Trust) {
	case "trusted", "untrusted", "approved":
	case "":
	default:
		errs.Add(prefix+".trust", "must be one of trusted, untrusted, or approved", "pick a supported trust tier")
	}
}

func validateMemoryReplaySession(errs *ValidationErrors, prefix string, session core.MemoryReplaySession) {
	requireNonEmpty(errs, prefix+".session_id", session.SessionID, "set the traced session identifier for this ordered replay step")
	if rawSessionID, ok := session.Metadata["session_id"]; ok {
		metadataSessionID := strings.TrimSpace(rawSessionID)
		switch {
		case metadataSessionID == "":
			errs.Add(prefix+".metadata.session_id", "cannot be empty when set", "remove metadata.session_id or set it to the same fixed value as session_id")
		case !strings.EqualFold(metadataSessionID, strings.TrimSpace(session.SessionID)):
			errs.Add(prefix+".metadata.session_id", "must match "+prefix+".session_id", "use one stable traced session_id value for the replay step")
		}
	}
	for i, source := range session.ContextSources {
		validateContextSource(errs, fmt.Sprintf("%s.context_sources[%d]", prefix, i), source)
	}
}

func validateExpectedMutation(errs *ValidationErrors, prefix string, mutation core.ExpectedMutation) {
	requireNonEmpty(errs, prefix+".path", mutation.Path, "set the expected file path that should be created, modified, or deleted")
	requireNonEmpty(errs, prefix+".kind", mutation.Kind, "set the expected mutation kind: created, modified, or deleted")

	switch strings.TrimSpace(mutation.Kind) {
	case "created", "modified", "deleted":
	case "":
	default:
		errs.Add(prefix+".kind", "must be one of created, modified, or deleted", "pick a supported file mutation kind")
	}

	if strings.TrimSpace(mutation.Kind) == "deleted" && strings.TrimSpace(mutation.ContentContains) != "" {
		errs.Add(prefix+".content_contains", "cannot be set when kind is deleted", "remove content_contains for deleted-file expectations")
	}
}

func validateExpectedStateChange(errs *ValidationErrors, prefix string, change core.ExpectedStateChange) {
	if strings.TrimSpace(change.Kind) == "" &&
		strings.TrimSpace(change.Target) == "" &&
		strings.TrimSpace(change.Action) == "" &&
		strings.TrimSpace(change.Status) == "" &&
		strings.TrimSpace(change.SummaryContains) == "" {
		errs.Add(prefix, "must declare at least one selector", "set kind, target, action, status, or summary_contains so cleanr can match an observed state change")
	}
}

func validatePolicyRule(errs *ValidationErrors, prefix string, rule core.PolicyRule) {
	ruleType := strings.TrimSpace(rule.Type)
	mode := strings.TrimSpace(rule.Mode)
	switch ruleType {
	case "tool":
		validateToolPolicyRule(errs, prefix, mode, rule)
	case "state_change":
		validateStateChangePolicyRule(errs, prefix, mode, rule)
	case "trust":
		validateTrustPolicyRule(errs, prefix, mode, rule)
	case "sink":
		validateSinkPolicyRule(errs, prefix, mode, rule)
	default:
		errs.Add(prefix+".type", "must be one of tool, state_change, trust, or sink", "pick a supported release-policy rule type")
	}

	validatePolicySeverity(errs, prefix, rule.Severity)
}

func validateToolPolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if !matchesLiteral(mode, "allow", "deny", "require_approval", "read_only") {
		errs.Add(prefix+".mode", "must be one of allow, deny, require_approval, or read_only", "pick a supported tool policy mode")
	}
	if len(rule.Tools) == 0 {
		errs.Add(prefix+".tools", "must contain at least one tool name", "set the tools this policy rule should match")
	}
}

func validateStateChangePolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if !matchesLiteral(mode, "allow", "deny", "require_approval") {
		errs.Add(prefix+".mode", "must be one of allow, deny, or require_approval", "pick a supported state-change policy mode")
	}
	if len(rule.StateKinds) == 0 && len(rule.StateActions) == 0 && len(rule.Targets) == 0 {
		errs.Add(prefix, "must declare at least one state selector", "set state_kinds, state_actions, or targets so cleanr can match observed state changes")
	}
}

func validateTrustPolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if !matchesLiteral(mode, "deny", "require_approval") {
		errs.Add(prefix+".mode", "must be one of deny or require_approval", "pick a supported trust-boundary policy mode")
	}
	if len(rule.Trusts) == 0 {
		errs.Add(prefix+".trusts", "must contain at least one trust tier", "set one or more trust values such as untrusted or approved")
	}
	if len(rule.Tools) == 0 && len(rule.StateKinds) == 0 && len(rule.StateActions) == 0 && len(rule.Targets) == 0 {
		errs.Add(prefix, "must declare an action selector", "set tools and/or state selectors so cleanr knows which actions the trust rule governs")
	}
}

func validateSinkPolicyRule(errs *ValidationErrors, prefix, mode string, rule core.PolicyRule) {
	if mode != "approved_only" {
		errs.Add(prefix+".mode", "must be approved_only", "use approved_only to restrict which sink tools may receive sensitive payload")
	}
	if len(rule.ApprovedTools) == 0 {
		errs.Add(prefix+".approved_tools", "must contain at least one tool name", "set the sink tools that are allowed to receive sensitive payload")
	}
}

func validatePolicySeverity(errs *ValidationErrors, prefix, severity string) {
	trimmed := strings.TrimSpace(severity)
	if trimmed == "" {
		return
	}
	if !matchesLiteral(trimmed, "low", "medium", "high", "critical") {
		errs.Add(prefix+".severity", "must be one of low, medium, high, or critical", "omit severity to use the default, or pick a supported severity level")
	}
}

func matchesLiteral(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func validateResultSink(errs *ValidationErrors, prefix string, sink core.ResultSinkConfig) {
	switch strings.TrimSpace(sink.Type) {
	case "http", "braintrust", "langfuse", "posthog":
	default:
		errs.Add(prefix+".type", "must be one of http, braintrust, langfuse, or posthog", "use http for a generic JSON webhook, braintrust for a Braintrust-style run publisher, langfuse for a Langfuse trace publisher, or posthog for a PostHog event publisher")
	}
	switch strings.TrimSpace(sink.Type) {
	case "http":
		requireNonEmpty(errs, prefix+".endpoint", sink.Endpoint, "set the remote endpoint that should receive the machine-readable cleanr result payload")
	case "braintrust":
		if strings.TrimSpace(sink.Endpoint) == "" && strings.TrimSpace(sink.Project) == "" {
			errs.Add(prefix, "must set endpoint or project", "set endpoint for a Braintrust-compatible webhook, or set project to use the native Braintrust API connector")
		}
		if strings.TrimSpace(sink.Project) == "" && strings.TrimSpace(sink.Endpoint) != "" {
			// Legacy webhook mode is allowed for backward compatibility.
		}
	case "langfuse":
		requireNonEmpty(errs, prefix+".public_key_env", sink.PublicKeyEnv, "set the env var that contains the Langfuse public key")
		requireNonEmpty(errs, prefix+".secret_key_env", sink.SecretKeyEnv, "set the env var that contains the Langfuse secret key")
	case "posthog":
		requireNonEmpty(errs, prefix+".project_token_env", sink.ProjectTokenEnv, "set the env var that contains the PostHog project API token")
	}
	if rawURL := strings.TrimSpace(sink.Endpoint); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			errs.Add(prefix+".endpoint", "must be an absolute http(s) URL", "use a value such as https://example.internal/cleanr/runs")
		}
	}
	if rawURL := strings.TrimSpace(sink.BaseURL); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			errs.Add(prefix+".base_url", "must be an absolute http(s) URL", "use a value such as https://api.braintrust.dev, https://cloud.langfuse.com, or your self-hosted provider base URL")
		}
	}
	if sink.TimeoutMS < 0 {
		errs.Add(prefix+".timeout_ms", "must be >= 0", "use a non-negative timeout in milliseconds")
	}
}

func validateTrendSource(errs *ValidationErrors, prefix string, source core.TrendSourceConfig) {
	switch strings.TrimSpace(source.Type) {
	case "file":
		requireNonEmpty(errs, prefix+".path", source.Path, "set the path to a retained cleanr trend history file")
	case "http":
		requireNonEmpty(errs, prefix+".url", source.URL, "set the remote URL that returns a cleanr trend history file")
		if rawURL := strings.TrimSpace(source.URL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".url", "must be an absolute http(s) URL", "use a value such as https://example.internal/cleanr/history.json")
			}
		}
	case "braintrust":
		requireNonEmpty(errs, prefix+".project", source.Project, "set the Braintrust project name that stores cleanr release-gate experiments")
		if rawURL := strings.TrimSpace(source.BaseURL); rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				errs.Add(prefix+".base_url", "must be an absolute http(s) URL", "use a value such as https://api.braintrust.dev or your Braintrust data plane URL")
			}
		}
	default:
		errs.Add(prefix+".type", "must be one of file, http, or braintrust", "use file for a retained local artifact, http for a remote history endpoint, or braintrust for native Braintrust experiment history")
	}
	if strings.TrimSpace(source.ViewURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(source.ViewURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			errs.Add(prefix+".view_url", "must be an absolute http(s) URL", "use a direct dashboard URL that reviewers can open when triaging the linked remote experiment")
		}
	}
	if source.HistoryLimit < 0 {
		errs.Add(prefix+".history_limit", "must be >= 0", "use 0 to keep the default remote history window, or set a positive retained-run limit")
	}
	if source.TimeoutMS < 0 {
		errs.Add(prefix+".timeout_ms", "must be >= 0", "use a non-negative timeout in milliseconds")
	}
}

func validateSummary(errs *ValidationErrors, prefix string, summary core.SummaryConfig) {
	switch strings.TrimSpace(summary.Format) {
	case "", "markdown", "json":
	default:
		errs.Add(prefix+".format", "must be one of markdown or json", "use markdown for PR or release notes, or json for downstream automation")
	}
	requireNonEmpty(errs, prefix+".output", summary.Output, "set the output path for the generated PR or release summary artifact")
}
