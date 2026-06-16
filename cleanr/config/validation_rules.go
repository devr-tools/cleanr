package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func validateTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	hints := targetHints(prefix)
	switch cfg.TargetType() {
	case "cli":
		validateCLITargetConfig(errs, prefix, cfg)
	case "graphql":
		validateGraphQLTargetConfig(errs, prefix, cfg, hints)
	case "grpc":
		validateGRPCTargetConfig(errs, prefix, cfg)
	case "http":
		validateHTTPTargetConfig(errs, prefix, cfg, hints)
	case "openai", "openai_compatible", "azure_openai", "gemini", "bedrock", "vertex", "mistral":
		validateOpenAITargetConfig(errs, prefix, cfg)
	case "anthropic":
		validateAnthropicTargetConfig(errs, prefix, cfg)
	case "mcp":
		validateMCPTargetConfig(errs, prefix, cfg)
	default:
		errs.Add(prefix+".type", "must be one of cli, graphql, grpc, http, openai, openai_compatible, azure_openai, gemini, bedrock, vertex, mistral, anthropic, or mcp", "set the target type to cli, graphql, grpc, http, openai, openai_compatible, azure_openai, gemini, bedrock, vertex, mistral, anthropic, or mcp")
	}
}

type targetValidationHints struct {
	url      string
	prompt   string
	response string
}

func validateCLITargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	requireNonEmpty(errs, prefix+".cli.command", cfg.CLI.Command, "set the executable path or command name to run for each scenario")
}

func validateGraphQLTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig, hints targetValidationHints) {
	requireNonEmpty(errs, prefix+".url", cfg.URL, hints.url)
	requireNonEmpty(errs, prefix+".graphql.query", cfg.GraphQL.Query, "set the GraphQL document to execute for each scenario")
	requireNonEmpty(errs, prefix+".response_field", cfg.ResponseField, hints.response)
	validateAbsoluteURL(errs, prefix+".url", cfg.URL, "use a value such as https://api.example.com/graphql or http://localhost:4000/graphql")
}

func validateGRPCTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	requireNonEmpty(errs, prefix+".grpc.address", cfg.GRPC.Address, "set the gRPC server address, for example 127.0.0.1:50051")
	requireNonEmpty(errs, prefix+".grpc.method", cfg.GRPC.Method, "set the fully-qualified gRPC method such as grpc.testing.TestService/UnaryCall")
}

func targetHints(prefix string) targetValidationHints {
	if prefix == "target" {
		return targetValidationHints{
			url:      "set target.url to the full API endpoint URL",
			prompt:   "set target.prompt_field to the request field that receives the prompt text",
			response: "set target.response_field to the JSON path that contains the model text response",
		}
	}
	return targetValidationHints{
		url:      "set the full API endpoint URL",
		prompt:   "set the request field that receives the prompt text",
		response: "set the JSON path that contains the model text response",
	}
}

func validateHTTPTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig, hints targetValidationHints) {
	requireNonEmpty(errs, prefix+".url", cfg.URL, hints.url)
	requireNonEmpty(errs, prefix+".prompt_field", cfg.PromptField, hints.prompt)
	requireNonEmpty(errs, prefix+".response_field", cfg.ResponseField, hints.response)
	validateAbsoluteURL(errs, prefix+".url", cfg.URL, "use a value such as http://localhost:8080/v1/chat or https://api.example.com/v1/chat")
}

func validateOpenAITargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	requireNonEmpty(errs, prefix+".openai.model", cfg.OpenAI.Model, "set the OpenAI model name, for example gpt-4o-mini or gpt-4.1-mini")
	validateOpenAIAPIMode(errs, prefix, cfg)
	validateAbsoluteURL(errs, prefix+".openai.base_url", cfg.OpenAI.BaseURL, "use a value such as https://api.openai.com/v1 or a compatible base URL for testing")
	if strings.TrimSpace(cfg.OpenAI.AuthHeader) == "" && cfg.TargetType() == "openai_compatible" {
		errs.Add(prefix+".openai.auth_header", "is required for openai_compatible targets", "set the auth header name, usually Authorization or api-key")
	}
	switch cfg.TargetType() {
	case "azure_openai", "vertex", "bedrock":
		requireNonEmpty(errs, prefix+".openai.base_url", cfg.OpenAI.BaseURL, "set the provider endpoint base URL, including any required deployment or gateway path")
	}
	if cfg.TargetType() == "azure_openai" {
		requireNonEmpty(errs, prefix+".openai.api_version", cfg.OpenAI.APIVersion, "set the Azure OpenAI api_version, for example 2025-03-01-preview")
	}
}

func validateOpenAIAPIMode(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	switch cfg.OpenAI.APIModeValue() {
	case "responses", "chat_completions":
	default:
		errs.Add(prefix+".openai.api_mode", "must be one of responses or chat_completions", "use responses for new projects or chat_completions for legacy-compatible message requests")
	}
}

func validateAnthropicTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	requireNonEmpty(errs, prefix+".anthropic.model", cfg.Anthropic.Model, "set the Anthropic model name, for example claude-sonnet-4-20250514")
	validateAbsoluteURL(errs, prefix+".anthropic.base_url", cfg.Anthropic.BaseURL, "use a value such as https://api.anthropic.com/v1 or a compatible base URL for testing")
	if cfg.Anthropic.MaxTokens < 0 {
		errs.Add(prefix+".anthropic.max_tokens", "must be >= 0", "set a positive max_tokens budget or omit the field to use the default")
	}
}

func validateMCPTargetConfig(errs *ValidationErrors, prefix string, cfg core.TargetConfig) {
	requireNonEmpty(errs, prefix+".mcp.url", cfg.MCP.URL, "set the MCP HTTP JSON-RPC endpoint URL")
	requireNonEmpty(errs, prefix+".mcp.tool", cfg.MCP.Tool, "set the MCP tool name to call for each scenario")
	validateAbsoluteURL(errs, prefix+".mcp.url", cfg.MCP.URL, "use a value such as http://localhost:8080/mcp or https://example.com/mcp")
}

func validateAbsoluteURL(errs *ValidationErrors, path, rawURL, hint string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		errs.Add(path, "must be an absolute http(s) URL", hint)
	}
}

func validateOpenAPISource(errs *ValidationErrors, prefix string, source core.OpenAPISource, hint string) {
	count := 0
	if strings.TrimSpace(source.Path) != "" {
		count++
	}
	if strings.TrimSpace(source.URL) != "" {
		count++
		validateAbsoluteURL(errs, prefix+".url", source.URL, "use an absolute http(s) URL to a reachable OpenAPI document")
	}
	if source.Inline != nil {
		count++
	}
	if count == 0 {
		errs.Add(prefix, "is required", hint)
		return
	}
	if count > 1 {
		errs.Add(prefix, "must set exactly one of path, url, or inline", "keep a single OpenAPI source so cleanr loads one unambiguous contract document")
	}
}

func validateOpenAPIMethodList(errs *ValidationErrors, path string, methods []string) {
	for i, method := range methods {
		switch strings.ToUpper(strings.TrimSpace(method)) {
		case "", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
			if strings.TrimSpace(method) == "" {
				errs.Add(fmt.Sprintf("%s[%d]", path, i), "cannot be empty", "remove empty values or set a concrete HTTP method such as GET or POST")
			}
		default:
			errs.Add(fmt.Sprintf("%s[%d]", path, i), "must be a supported HTTP method", "use GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, or TRACE")
		}
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
	assertionType := strings.TrimSpace(assertion.Type)
	switch assertionType {
	case "contains", "not_contains":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the text fragment the response should include or exclude")
	case "regex":
		validateRegexAssertion(errs, prefix, assertion)
	case "json_schema":
		validateJSONSchemaAssertion(errs, prefix, assertion)
	case "json_path":
		requireNonEmpty(errs, prefix+".path", assertion.Path, "set the response path to check, for example response.provider_model or response.body.output.0.content.0.text")
	case "status_code", "exit_code":
		validateCodeAssertion(errs, prefix, assertionType, assertion.IntValue)
	case "latency_ms", "stream_ttft_ms", "stream_duration_ms", "stream_chunk_cadence_ms":
		validateThresholdAssertion(errs, prefix, assertion.IntValue)
	case "finish_reason", "tool_call_name":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the expected provider finish reason or tool name")
	case "stream_tool_call_name":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set the expected streamed tool name")
	case "tool_call_count":
		validateToolCallCountAssertion(errs, prefix, assertion.IntValue)
	case "tool_call_order":
		requireNonEmpty(errs, prefix+".value", assertion.Value, "set a comma-separated ordered tool list such as lookup_policy, create_ticket")
	case "tool_call_arguments_schema":
		requireNonEmpty(errs, prefix+".path", assertion.Path, "set the tool call path to validate, for example response.tool_calls.0 or response.tool_calls.0.parsed_arguments")
		validateJSONSchemaAssertion(errs, prefix, assertion)
	default:
		errs.Add(prefix+".type", "must be one of contains, not_contains, regex, json_schema, json_path, status_code, exit_code, latency_ms, stream_ttft_ms, stream_duration_ms, stream_chunk_cadence_ms, finish_reason, tool_call_count, tool_call_name, stream_tool_call_name, tool_call_order, or tool_call_arguments_schema", "pick one of the built-in assertion types")
	}

	if severity := strings.TrimSpace(assertion.Severity); severity != "" {
		switch severity {
		case "low", "medium", "high", "critical":
		default:
			errs.Add(prefix+".severity", "must be one of low, medium, high, or critical", "omit severity to use the default, or pick a supported severity level")
		}
	}
}

func validateRegexAssertion(errs *ValidationErrors, prefix string, assertion core.Assertion) {
	requireNonEmpty(errs, prefix+".pattern", assertion.Pattern, "set a valid Go regular expression to match against the response field")
	if strings.TrimSpace(assertion.Pattern) == "" {
		return
	}
	if _, err := regexp.Compile(assertion.Pattern); err != nil {
		errs.Add(prefix+".pattern", "must be a valid Go regular expression", "fix the pattern syntax or remove the assertion")
	}
}

func validateJSONSchemaAssertion(errs *ValidationErrors, prefix string, assertion core.Assertion) {
	if assertion.Schema == nil {
		errs.Add(prefix+".schema", "is required", "set an inline JSON Schema object to validate the selected response payload")
	}
}

func validateCodeAssertion(errs *ValidationErrors, prefix, assertionType string, intValue *int) {
	if intValue == nil {
		hint := "set the expected HTTP status code such as 200"
		if assertionType == "exit_code" {
			hint = "set the expected process exit code such as 0"
		}
		errs.Add(prefix+".int_value", "is required", hint)
		return
	}
	switch assertionType {
	case "status_code":
		if *intValue < 100 || *intValue > 599 {
			errs.Add(prefix+".int_value", "must be between 100 and 599", "use a valid HTTP status code")
		}
	case "exit_code":
		if *intValue < 0 {
			errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative process exit code")
		}
	}
}

func validateThresholdAssertion(errs *ValidationErrors, prefix string, intValue *int) {
	if intValue == nil {
		errs.Add(prefix+".int_value", "is required", "set the maximum allowed latency in milliseconds")
		return
	}
	if *intValue < 0 {
		errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative millisecond threshold")
	}
}

func validateToolCallCountAssertion(errs *ValidationErrors, prefix string, intValue *int) {
	if intValue == nil {
		errs.Add(prefix+".int_value", "is required", "set the expected number of tool calls, including 0 when no tool calls should be present")
		return
	}
	if *intValue < 0 {
		errs.Add(prefix+".int_value", "must be >= 0", "use a non-negative expected tool call count")
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

func validateConversationTurn(errs *ValidationErrors, prefix string, turn core.ConversationTurn) {
	requireNonEmpty(errs, prefix+".role", turn.Role, "set the turn role, for example system, user, assistant, or tool")
	if strings.TrimSpace(turn.Content) == "" && len(turn.Images) == 0 && len(turn.Audio) == 0 && len(turn.PDFs) == 0 {
		errs.Add(prefix+".content", "is required", "set the message content or attach images, audio, or pdfs for this transcript turn")
	}
	for i, item := range turn.Images {
		validateMediaInput(errs, fmt.Sprintf("%s.images[%d]", prefix, i), "image", item)
	}
	for i, item := range turn.Audio {
		validateMediaInput(errs, fmt.Sprintf("%s.audio[%d]", prefix, i), "audio", item)
	}
	for i, item := range turn.PDFs {
		validateMediaInput(errs, fmt.Sprintf("%s.pdfs[%d]", prefix, i), "pdf", item)
	}
	for i, item := range turn.MockToolResults {
		validateMockToolResult(errs, fmt.Sprintf("%s.mock_tool_results[%d]", prefix, i), item)
	}

	switch strings.TrimSpace(turn.Role) {
	case "system", "user", "assistant", "tool":
	case "":
	default:
		errs.Add(prefix+".role", "must be one of system, user, assistant, or tool", "pick a supported transcript role")
	}
	if strings.TrimSpace(turn.Role) == "tool" {
		requireNonEmpty(errs, prefix+".name", turn.Name, "set the tool name for tool transcript turns")
	}
}

func validateMockToolResult(errs *ValidationErrors, prefix string, item core.MockToolResult) {
	requireNonEmpty(errs, prefix+".name", item.Name, "set the mocked tool name such as lookup_policy or fetch_customer")
	if strings.TrimSpace(item.Content) == "" && len(item.Images) == 0 && len(item.Audio) == 0 && len(item.PDFs) == 0 {
		errs.Add(prefix+".content", "is required", "set the mocked tool result payload or attach mocked media output")
	}
	for i, media := range item.Images {
		validateMediaInput(errs, fmt.Sprintf("%s.images[%d]", prefix, i), "image", media)
	}
	for i, media := range item.Audio {
		validateMediaInput(errs, fmt.Sprintf("%s.audio[%d]", prefix, i), "audio", media)
	}
	for i, media := range item.PDFs {
		validateMediaInput(errs, fmt.Sprintf("%s.pdfs[%d]", prefix, i), "pdf", media)
	}
}

func validateMediaInput(errs *ValidationErrors, prefix, kind string, item core.MediaInput) {
	if strings.TrimSpace(item.URL) == "" && strings.TrimSpace(item.Path) == "" && strings.TrimSpace(item.Data) == "" {
		errs.Add(prefix, "must set url, path, or data", "point the media input at a URL or local path, or provide inline data")
	}
	if strings.TrimSpace(item.Detail) != "" {
		switch strings.ToLower(strings.TrimSpace(item.Detail)) {
		case "low", "high", "auto":
		default:
			errs.Add(prefix+".detail", "must be one of low, high, or auto", "omit detail or use a supported image detail hint")
		}
	}
	if kind == "pdf" && strings.TrimSpace(item.MediaType) == "" {
		errs.Add(prefix+".media_type", "is recommended for pdf inputs", "set media_type to application/pdf for portability across providers")
	}
}

func validateJudgeOutput(errs *ValidationErrors, prefix string, item core.JudgeOutput) {
	requireNonEmpty(errs, prefix+".type", item.Type, "set the output type to image, audio, or pdf so the judge knows what artifact to inspect")
	requireNonEmpty(errs, prefix+".path", item.Path, "set the response path that resolves to the generated artifact reference, for example response.body.output.0.url")
	if value := strings.TrimSpace(item.Value); value != "" {
		errs.Add(prefix+".value", "is runtime-only", "remove value from config; cleanr populates it after resolving the configured response path")
	}
	switch strings.ToLower(strings.TrimSpace(item.Type)) {
	case "image", "audio", "pdf", "":
	default:
		errs.Add(prefix+".type", "must be one of image, audio, or pdf", "pick the artifact family the judge should inspect")
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
		validateFileTrendSource(errs, prefix, source)
	case "http":
		validateHTTPTrendSource(errs, prefix, source)
	case "braintrust":
		validateBraintrustTrendSource(errs, prefix, source)
	case "langsmith", "openllmetry", "provider_logs":
		validateImportedTrendSource(errs, prefix, source)
	default:
		errs.Add(prefix+".type", "must be one of file, http, braintrust, langsmith, openllmetry, or provider_logs", "use file for a retained local artifact, http for a remote history endpoint, braintrust for native Braintrust experiment history, or a vendor log source for embedded cleanr traces")
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

func validateFileTrendSource(errs *ValidationErrors, prefix string, source core.TrendSourceConfig) {
	requireNonEmpty(errs, prefix+".path", source.Path, "set the path to a retained cleanr trend history file")
}

func validateHTTPTrendSource(errs *ValidationErrors, prefix string, source core.TrendSourceConfig) {
	requireNonEmpty(errs, prefix+".url", source.URL, "set the remote URL that returns a cleanr trend history file")
	validateAbsoluteURL(errs, prefix+".url", source.URL, "use a value such as https://example.internal/cleanr/history.json")
}

func validateBraintrustTrendSource(errs *ValidationErrors, prefix string, source core.TrendSourceConfig) {
	requireNonEmpty(errs, prefix+".project", source.Project, "set the Braintrust project name that stores cleanr release-gate experiments")
	validateAbsoluteURL(errs, prefix+".base_url", source.BaseURL, "use a value such as https://api.braintrust.dev or your Braintrust data plane URL")
}

func validateImportedTrendSource(errs *ValidationErrors, prefix string, source core.TrendSourceConfig) {
	if strings.TrimSpace(source.Path) == "" && strings.TrimSpace(source.URL) == "" {
		errs.Add(prefix, "must set path or url", "set a local export path or remote endpoint that returns normalized cleanr rows or vendor records with embedded cleanr metadata")
	}
	validateAbsoluteURL(errs, prefix+".url", source.URL, "use a value such as https://api.smith.langchain.com/runs/export, https://collector.internal/logs, or another reachable JSON endpoint")
}

func validateSummary(errs *ValidationErrors, prefix string, summary core.SummaryConfig) {
	switch strings.TrimSpace(summary.Format) {
	case "", "markdown", "json":
	default:
		errs.Add(prefix+".format", "must be one of markdown or json", "use markdown for PR or release notes, or json for downstream automation")
	}
	requireNonEmpty(errs, prefix+".output", summary.Output, "set the output path for the generated PR or release summary artifact")
}
