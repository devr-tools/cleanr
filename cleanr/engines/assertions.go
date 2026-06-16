package engines

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type assertionEvaluator func(core.Assertion, map[string]any, core.Response) (core.Finding, bool)

var assertionEvaluators = map[string]assertionEvaluator{
	"contains":                   evaluateContainsAssertionWithResponse,
	"not_contains":               evaluateNotContainsAssertionWithResponse,
	"regex":                      evaluateRegexAssertionWithResponse,
	"json_schema":                evaluateJSONSchemaAssertionWithResponse,
	"json_path":                  evaluateJSONPathAssertionWithResponse,
	"status_code":                evaluateStatusCodeAssertionWithResponse,
	"exit_code":                  evaluateExitCodeAssertionWithResponse,
	"latency_ms":                 evaluateLatencyAssertionWithResponse,
	"stream_ttft_ms":             evaluateStreamTTFTAssertionWithResponse,
	"stream_duration_ms":         evaluateStreamDurationAssertionWithResponse,
	"stream_chunk_cadence_ms":    evaluateStreamChunkCadenceAssertionWithResponse,
	"finish_reason":              evaluateFinishReasonAssertionWithResponse,
	"tool_call_count":            evaluateToolCallCountAssertionWithResponse,
	"tool_call_name":             evaluateToolCallNameAssertionWithResponse,
	"stream_tool_call_name":      evaluateStreamToolCallNameAssertionWithResponse,
	"tool_call_order":            evaluateToolCallOrderAssertionWithResponse,
	"tool_call_arguments_schema": evaluateToolCallArgumentsSchemaAssertionWithResponse,
}

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
	evaluator, ok := assertionEvaluators[strings.TrimSpace(assertion.Type)]
	if !ok {
		return core.Finding{}, false
	}
	return evaluator(assertion, view, resp)
}

func evaluateContainsAssertionWithResponse(assertion core.Assertion, view map[string]any, _ core.Response) (core.Finding, bool) {
	return evaluateContainsAssertion(assertion, view)
}

func evaluateNotContainsAssertionWithResponse(assertion core.Assertion, view map[string]any, _ core.Response) (core.Finding, bool) {
	return evaluateNotContainsAssertion(assertion, view)
}

func evaluateRegexAssertionWithResponse(assertion core.Assertion, view map[string]any, _ core.Response) (core.Finding, bool) {
	return evaluateRegexAssertion(assertion, view)
}

func evaluateJSONSchemaAssertionWithResponse(assertion core.Assertion, view map[string]any, _ core.Response) (core.Finding, bool) {
	return evaluateJSONSchemaAssertion(assertion, view)
}

func evaluateJSONPathAssertionWithResponse(assertion core.Assertion, view map[string]any, _ core.Response) (core.Finding, bool) {
	return evaluateJSONPathAssertion(assertion, view)
}

func evaluateStatusCodeAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateStatusCodeAssertion(assertion, resp)
}

func evaluateExitCodeAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateExitCodeAssertion(assertion, resp)
}

func evaluateLatencyAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateLatencyAssertion(assertion, resp)
}

func evaluateStreamTTFTAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateStreamTTFTAssertion(assertion, resp)
}

func evaluateStreamDurationAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateStreamDurationAssertion(assertion, resp)
}

func evaluateStreamChunkCadenceAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateStreamChunkCadenceAssertion(assertion, resp)
}

func evaluateFinishReasonAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateFinishReasonAssertion(assertion, resp)
}

func evaluateToolCallCountAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateToolCallCountAssertion(assertion, resp)
}

func evaluateToolCallNameAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateToolCallNameAssertion(assertion, resp)
}

func evaluateStreamToolCallNameAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateStreamToolCallNameAssertion(assertion, resp)
}

func evaluateToolCallOrderAssertionWithResponse(assertion core.Assertion, _ map[string]any, resp core.Response) (core.Finding, bool) {
	return evaluateToolCallOrderAssertion(assertion, resp)
}

func evaluateToolCallArgumentsSchemaAssertionWithResponse(assertion core.Assertion, view map[string]any, _ core.Response) (core.Finding, bool) {
	return evaluateToolCallArgumentsSchemaAssertion(assertion, view)
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

func evaluateJSONSchemaAssertion(assertion core.Assertion, view map[string]any) (core.Finding, bool) {
	path := strings.TrimSpace(assertion.Path)
	if path == "" {
		path = "response.body"
	}
	actual, ok := resolveAssertionPath(view, path)
	if !ok {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: %s was not present", path)), true
	}
	if err := validateJSONSchema(path, actual, assertion.Schema); err != nil {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: %v", err)), true
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

func validateJSONSchema(path string, actual, schema any) error {
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("%s schema must be an object", path)
	}
	validators := []func(string, any, map[string]any) error{
		func(path string, actual any, schema map[string]any) error {
			return validateJSONSchemaType(path, actual, schema["type"])
		},
		func(path string, actual any, schema map[string]any) error {
			return validateJSONSchemaConst(path, actual, schema["const"])
		},
		func(path string, actual any, schema map[string]any) error {
			return validateJSONSchemaEnum(path, actual, schema["enum"])
		},
		validateJSONSchemaNumber,
		validateJSONSchemaString,
		validateJSONSchemaArray,
		validateJSONSchemaObject,
	}
	for _, validator := range validators {
		if err := validator(path, actual, schemaMap); err != nil {
			return err
		}
	}
	return nil
}

func validateJSONSchemaType(path string, actual, schemaType any) error {
	expected := strings.TrimSpace(renderAssertionValue(schemaType))
	if expected == "" {
		return nil
	}
	actualType := jsonSchemaType(actual)
	if actualType != expected {
		return fmt.Errorf("%s expected type %q, got %q", path, expected, actualType)
	}
	return nil
}

func validateJSONSchemaConst(path string, actual, schemaConst any) error {
	if schemaConst == nil {
		return nil
	}
	if !reflect.DeepEqual(normalizeAssertionValue(actual), normalizeAssertionValue(schemaConst)) {
		return fmt.Errorf("%s expected const %s, got %s", path, renderAssertionValue(schemaConst), renderAssertionValue(actual))
	}
	return nil
}

func validateJSONSchemaEnum(path string, actual, enumValue any) error {
	items, ok := enumValue.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	normalizedActual := normalizeAssertionValue(actual)
	for _, item := range items {
		if reflect.DeepEqual(normalizedActual, normalizeAssertionValue(item)) {
			return nil
		}
	}
	return fmt.Errorf("%s expected one of %s, got %s", path, renderAssertionValue(items), renderAssertionValue(actual))
}

func validateJSONSchemaNumber(path string, actual any, schema map[string]any) error {
	actualNumber, ok := jsonSchemaNumber(actual)
	if !ok {
		return nil
	}
	if minimum, ok := jsonSchemaNumber(schema["minimum"]); ok && actualNumber < minimum {
		return fmt.Errorf("%s expected minimum %s, got %s", path, trimFloat(minimum), trimFloat(actualNumber))
	}
	if maximum, ok := jsonSchemaNumber(schema["maximum"]); ok && actualNumber > maximum {
		return fmt.Errorf("%s expected maximum %s, got %s", path, trimFloat(maximum), trimFloat(actualNumber))
	}
	return nil
}

func validateJSONSchemaString(path string, actual any, schema map[string]any) error {
	actualString, ok := actual.(string)
	if !ok {
		return nil
	}
	if minLength, ok := jsonSchemaInteger(schema["minLength"]); ok && len(actualString) < minLength {
		return fmt.Errorf("%s expected minLength %d, got %d", path, minLength, len(actualString))
	}
	if maxLength, ok := jsonSchemaInteger(schema["maxLength"]); ok && len(actualString) > maxLength {
		return fmt.Errorf("%s expected maxLength %d, got %d", path, maxLength, len(actualString))
	}
	return nil
}

func validateJSONSchemaArray(path string, actual any, schema map[string]any) error {
	items, ok := actual.([]any)
	if !ok {
		return nil
	}
	if minItems, ok := jsonSchemaInteger(schema["minItems"]); ok && len(items) < minItems {
		return fmt.Errorf("%s expected minItems %d, got %d", path, minItems, len(items))
	}
	if maxItems, ok := jsonSchemaInteger(schema["maxItems"]); ok && len(items) > maxItems {
		return fmt.Errorf("%s expected maxItems %d, got %d", path, maxItems, len(items))
	}
	itemSchema, ok := schema["items"]
	if !ok || itemSchema == nil {
		return nil
	}
	for i, item := range items {
		if err := validateJSONSchema(fmt.Sprintf("%s.%d", path, i), item, itemSchema); err != nil {
			return err
		}
	}
	return nil
}

func validateJSONSchemaObject(path string, actual any, schema map[string]any) error {
	actualObject, ok := actual.(map[string]any)
	if !ok {
		return nil
	}
	if err := validateJSONSchemaRequired(path, actualObject, schema["required"]); err != nil {
		return err
	}
	properties, _ := schema["properties"].(map[string]any)
	for key, propertySchema := range properties {
		value, exists := actualObject[key]
		if !exists {
			continue
		}
		if err := validateJSONSchema(path+"."+key, value, propertySchema); err != nil {
			return err
		}
	}
	return validateJSONSchemaAdditionalProperties(path, actualObject, properties, schema["additionalProperties"])
}

func validateJSONSchemaRequired(path string, actualObject map[string]any, requiredValue any) error {
	required, ok := requiredValue.([]any)
	if !ok {
		return nil
	}
	for _, item := range required {
		key := strings.TrimSpace(renderAssertionValue(item))
		if key == "" {
			continue
		}
		if _, exists := actualObject[key]; !exists {
			return fmt.Errorf("%s missing required property %q", path, key)
		}
	}
	return nil
}

func validateJSONSchemaAdditionalProperties(path string, actualObject map[string]any, properties map[string]any, additionalProperties any) error {
	allowUnknown, ok := additionalProperties.(bool)
	if !ok || allowUnknown {
		return nil
	}
	for key := range actualObject {
		if _, exists := properties[key]; !exists {
			return fmt.Errorf("%s has unexpected property %q", path, key)
		}
	}
	return nil
}

func jsonSchemaType(value any) string {
	valueType := ""
	switch value.(type) {
	case nil:
		valueType = "null"
	case bool:
		valueType = "boolean"
	case string:
		valueType = "string"
	case []any:
		valueType = "array"
	case map[string]any:
		valueType = "object"
	default:
		if _, ok := jsonSchemaNumber(value); ok {
			valueType = "number"
		}
	}
	return valueType
}

func jsonSchemaNumber(value any) (float64, bool) {
	number := 0.0
	ok := true
	switch typed := value.(type) {
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case int32:
		number = float64(typed)
	case json.Number:
		f, err := typed.Float64()
		number = f
		ok = err == nil
	default:
		ok = false
	}
	return number, ok
}

func jsonSchemaInteger(value any) (int, bool) {
	number, ok := jsonSchemaNumber(value)
	if !ok || math.Trunc(number) != number {
		return 0, false
	}
	return int(number), true
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func evaluateStatusCodeAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && resp.StatusCode != *assertion.IntValue {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected status code %d, got %d", *assertion.IntValue, resp.StatusCode)), true
	}
	return core.Finding{}, false
}

func evaluateExitCodeAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && resp.ExitCode != *assertion.IntValue {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected exit code %d, got %d", *assertion.IntValue, resp.ExitCode)), true
	}
	return core.Finding{}, false
}

func evaluateLatencyAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && resp.Latency.Milliseconds() > int64(*assertion.IntValue) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected latency <= %dms, got %dms", *assertion.IntValue, resp.Latency.Milliseconds())), true
	}
	return core.Finding{}, false
}

func evaluateStreamTTFTAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && (resp.Stream.TTFTMS == 0 || resp.Stream.TTFTMS > int64(*assertion.IntValue)) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected stream ttft <= %dms, got %dms", *assertion.IntValue, resp.Stream.TTFTMS)), true
	}
	return core.Finding{}, false
}

func evaluateStreamDurationAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue != nil && (resp.Stream.DurationMS == 0 || resp.Stream.DurationMS > int64(*assertion.IntValue)) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected stream duration <= %dms, got %dms", *assertion.IntValue, resp.Stream.DurationMS)), true
	}
	return core.Finding{}, false
}

func evaluateStreamChunkCadenceAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if assertion.IntValue == nil {
		return core.Finding{}, false
	}
	if resp.Stream.ChunkCount < 2 || resp.Stream.DurationMS == 0 || resp.Stream.TTFTMS == 0 {
		return newAssertionFinding(assertion, "assertion failed: expected stream chunk cadence metrics, got incomplete stream timing"), true
	}
	cadenceMS := streamChunkCadenceMS(resp.Stream)
	if cadenceMS > int64(*assertion.IntValue) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected stream chunk cadence <= %dms, got %dms", *assertion.IntValue, cadenceMS)), true
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

func evaluateStreamToolCallNameAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	if resp.Stream.ChunkCount == 0 {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected streamed tool call %q, but response was not streamed", assertion.Value)), true
	}
	for _, toolCall := range resp.Normalized.ToolCalls {
		if toolCall.Name == assertion.Value {
			return core.Finding{}, false
		}
	}
	return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected streamed tool call %q", assertion.Value)), true
}

func evaluateToolCallOrderAssertion(assertion core.Assertion, resp core.Response) (core.Finding, bool) {
	expectedNames := parseAssertionCSV(assertion.Value)
	actualNames := make([]string, 0, len(resp.Normalized.ToolCalls))
	for _, toolCall := range resp.Normalized.ToolCalls {
		actualNames = append(actualNames, toolCall.Name)
	}
	if len(actualNames) != len(expectedNames) {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected tool call order [%s], got [%s]", strings.Join(expectedNames, ", "), strings.Join(actualNames, ", "))), true
	}
	for i := range expectedNames {
		if actualNames[i] != expectedNames[i] {
			return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: expected tool call order [%s], got [%s]", strings.Join(expectedNames, ", "), strings.Join(actualNames, ", "))), true
		}
	}
	return core.Finding{}, false
}

func evaluateToolCallArgumentsSchemaAssertion(assertion core.Assertion, view map[string]any) (core.Finding, bool) {
	actual, ok := resolveAssertionPath(view, assertion.Path)
	if !ok {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: %s was not present", assertion.Path)), true
	}
	if toolCall, ok := actual.(map[string]any); ok {
		if parsedArgs, exists := toolCall["parsed_arguments"]; exists {
			actual = parsedArgs
		}
	}
	if err := validateJSONSchema(assertion.Path, actual, assertion.Schema); err != nil {
		return newAssertionFinding(assertion, fmt.Sprintf("assertion failed: %v", err)), true
	}
	return core.Finding{}, false
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
			"stdout":                 resp.Text,
			"stderr":                 resp.Stderr,
			"status_code":            resp.StatusCode,
			"exit_code":              resp.ExitCode,
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
			"stream":                 normalizeAssertionValue(resp.Stream),
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
	rendered := ""
	switch typed := v.(type) {
	case nil:
	case string:
		rendered = typed
	case bool:
		if typed {
			rendered = "true"
		} else {
			rendered = "false"
		}
	case float64:
		rendered = strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		rendered = strconv.Itoa(typed)
	case int64:
		rendered = strconv.FormatInt(typed, 10)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			rendered = fmt.Sprint(typed)
		} else {
			rendered = string(data)
		}
	}
	return rendered
}

func streamChunkCadenceMS(stream core.StreamMetrics) int64 {
	if stream.ChunkCount < 2 {
		return 0
	}
	gapMS := stream.DurationMS - stream.TTFTMS
	if gapMS < 0 {
		gapMS = 0
	}
	return gapMS / int64(stream.ChunkCount-1)
}

func parseAssertionCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
