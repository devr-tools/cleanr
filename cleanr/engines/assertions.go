package engines

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func evaluateScenarioAssertions(scenario core.Scenario, resp core.Response) []core.Finding {
	assertions := scenarioAssertions(scenario)
	if len(assertions) == 0 {
		return nil
	}

	view := buildAssertionView(resp)
	findings := make([]core.Finding, 0)
	for _, assertion := range assertions {
		if finding, ok := evaluateAssertion(assertion, view, resp); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func scenarioAssertions(scenario core.Scenario) []core.Assertion {
	assertions := make([]core.Assertion, 0, len(scenario.Assertions)+len(scenario.ExpectedContains)+len(scenario.ForbiddenContains))
	assertions = append(assertions, scenario.Assertions...)
	for _, expected := range scenario.ExpectedContains {
		assertions = append(assertions, core.Assertion{
			Type:     "contains",
			Value:    expected,
			Severity: "medium",
			Message:  fmt.Sprintf("expected phrase missing: %s", expected),
		})
	}
	for _, forbidden := range scenario.ForbiddenContains {
		assertions = append(assertions, core.Assertion{
			Type:     "not_contains",
			Value:    forbidden,
			Severity: "critical",
			Message:  fmt.Sprintf("forbidden content detected: %s", forbidden),
		})
	}
	return assertions
}

func evaluateAssertion(assertion core.Assertion, view map[string]any, resp core.Response) (core.Finding, bool) {
	switch strings.TrimSpace(assertion.Type) {
	case "contains":
		return evaluateContainsAssertion(assertion, view)
	case "not_contains":
		return evaluateNotContainsAssertion(assertion, view)
	case "regex":
		return evaluateRegexAssertion(assertion, view)
	case "json_path":
		return evaluateJSONPathAssertion(assertion, view)
	case "status_code":
		return evaluateStatusCodeAssertion(assertion, resp)
	case "latency_ms":
		return evaluateLatencyAssertion(assertion, resp)
	case "finish_reason":
		return evaluateFinishReasonAssertion(assertion, resp)
	case "tool_call_count":
		return evaluateToolCallCountAssertion(assertion, resp)
	case "tool_call_name":
		return evaluateToolCallNameAssertion(assertion, resp)
	}
	return core.Finding{}, false
}

func evaluateContainsAssertion(assertion core.Assertion, view map[string]any) (core.Finding, bool) {
	path := defaultAssertionPath(assertion)
	actual, ok := resolveAssertionPath(view, path)
	if !ok || !strings.Contains(strings.ToLower(renderAssertionValue(actual)), strings.ToLower(assertion.Value)) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected %s to contain %q", path, assertion.Value)), true
	}
	return core.Finding{}, false
}

func evaluateNotContainsAssertion(assertion core.Assertion, view map[string]any) (core.Finding, bool) {
	path := defaultAssertionPath(assertion)
	actual, ok := resolveAssertionPath(view, path)
	if ok && strings.Contains(strings.ToLower(renderAssertionValue(actual)), strings.ToLower(assertion.Value)) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected %s not to contain %q", path, assertion.Value)), true
	}
	return core.Finding{}, false
}

func evaluateRegexAssertion(assertion core.Assertion, view map[string]any) (core.Finding, bool) {
	path := defaultAssertionPath(assertion)
	actual, ok := resolveAssertionPath(view, path)
	if !ok {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: %s was not present", path)), true
	}
	matched, err := regexp.MatchString(assertion.Pattern, renderAssertionValue(actual))
	if err != nil || !matched {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected %s to match %q", path, assertion.Pattern)), true
	}
	return core.Finding{}, false
}

func evaluateJSONPathAssertion(assertion core.Assertion, view map[string]any) (core.Finding, bool) {
	actual, ok := resolveAssertionPath(view, assertion.Path)
	if !ok {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: %s was not present", assertion.Path)), true
	}
	actualValue := renderAssertionValue(actual)
	if strings.TrimSpace(assertion.Value) != "" && actualValue != assertion.Value {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected %s to equal %q, got %q", assertion.Path, assertion.Value, actualValue)), true
	}
	return core.Finding{}, false
}

func evaluateStatusCodeAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && resp.StatusCode != *assertion.IntValue {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected status code %d, got %d", *assertion.IntValue, resp.StatusCode)), true
	}
	return core.Finding{}, false
}

func evaluateLatencyAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && resp.Latency.Milliseconds() > int64(*assertion.IntValue) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected latency <= %dms, got %dms", *assertion.IntValue, resp.Latency.Milliseconds())), true
	}
	return core.Finding{}, false
}

func evaluateFinishReasonAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if resp.Normalized.FinishReason != assertion.Value {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected finish reason %q, got %q", assertion.Value, resp.Normalized.FinishReason)), true
	}
	return core.Finding{}, false
}

func evaluateToolCallCountAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && len(resp.Normalized.ToolCalls) != *assertion.IntValue {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected %d tool calls, got %d", *assertion.IntValue, len(resp.Normalized.ToolCalls))), true
	}
	return core.Finding{}, false
}

func evaluateToolCallNameAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	for _, toolCall := range resp.Normalized.ToolCalls {
		if toolCall.Name == assertion.Value {
			return core.Finding{}, false
		}
	}
	return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected tool call %q", assertion.Value)), true
}

func defaultAssertionPath(assertion core.Assertion) string {
	if strings.TrimSpace(assertion.Path) != "" {
		return assertion.Path
	}
	return "response.text"
}

func newAssertionFinding(assertion core.Assertion, fallback string) core.Finding {
	message := strings.TrimSpace(assertion.Message)
	if message == "" {
		message = fallback
	}
	severity := strings.TrimSpace(assertion.Severity)
	if severity == "" {
		severity = "medium"
	}
	return core.Finding{
		Severity: severity,
		Message:  message,
	}
}

func buildAssertionView(resp core.Response) map[string]any {
	body := decodeJSONBody(resp.Body)
	return map[string]any{
		"response": map[string]any{
			"text":                   resp.Text,
			"status_code":            resp.StatusCode,
			"latency_ms":             resp.Latency.Milliseconds(),
			"body":                   body,
			"usage":                  normalizeAssertionValue(resp.Usage),
			"provider":               resp.Normalized.Provider,
			"provider_id":            resp.Normalized.ID,
			"provider_model":         resp.Normalized.Model,
			"provider_role":          resp.Normalized.Role,
			"provider_status":        resp.Normalized.Status,
			"finish_reason":          resp.Normalized.FinishReason,
			"stop_sequence":          resp.Normalized.StopSequence,
			"tool_call_count":        len(resp.Normalized.ToolCalls),
			"tool_calls":             normalizeAssertionValue(resp.Normalized.ToolCalls),
			"source_use_count":       len(resp.Normalized.SourceUses),
			"source_uses":            normalizeAssertionValue(resp.Normalized.SourceUses),
			"approval_count":         len(resp.Normalized.Approvals),
			"approvals":              normalizeAssertionValue(resp.Normalized.Approvals),
			"state_change_count":     len(resp.Normalized.StateChanges),
			"state_changes":          normalizeAssertionValue(resp.Normalized.StateChanges),
			"memory_operation_count": len(resp.Normalized.MemoryOperations),
			"memory_operations":      normalizeAssertionValue(resp.Normalized.MemoryOperations),
			"provider_raw":           normalizeAssertionValue(resp.Normalized.Raw),
		},
	}
}

func decodeJSONBody(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload
}

func normalizeAssertionValue(v any) any {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return v
	}
	return generic
}

func resolveAssertionPath(root any, path string) (any, bool) {
	current := root
	for _, segment := range strings.Split(strings.TrimSpace(path), ".") {
		if segment == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func renderAssertionValue(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}
