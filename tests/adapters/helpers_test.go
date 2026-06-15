package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stubDefaultTransport(t *testing.T, transport http.RoundTripper) func() {
	t.Helper()

	original := http.DefaultTransport
	http.DefaultTransport = transport
	return func() {
		http.DefaultTransport = original
	}
}

func jsonResponse(t *testing.T, statusCode int, body map[string]any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func sseResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func runConfigAsJSONReport(t *testing.T, configPath string) cleanr.Report {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.Run([]string{"run", "-config", configPath, "-format", "json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected run to succeed, got exit code %d, stdout=%s, stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	var report cleanr.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\nstdout=%s", err, stdout.String())
	}
	return report
}

func requirePassingSecurityReport(t *testing.T, report cleanr.Report, wantName string) {
	t.Helper()

	if report.Name != wantName {
		t.Fatalf("unexpected report name %q", report.Name)
	}
	if !report.Passed {
		t.Fatalf("expected passing report, got %+v", report)
	}
	if len(report.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %+v", report.Suites)
	}

	suite := report.Suites[0]
	if suite.Name != "security" {
		t.Fatalf("expected security suite, got %+v", suite)
	}
	if !suite.Passed {
		t.Fatalf("expected passing suite, got %+v", suite)
	}
	if len(suite.Cases) != 1 || !suite.Cases[0].Passed {
		t.Fatalf("expected one passing case, got %+v", suite.Cases)
	}
}

func decodeRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	defer r.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func assertChatCompletionsRequest(body map[string]any, wantSystem, wantPrompt string) error {
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		return fmt.Errorf("request missing model")
	}

	rawMessages, ok := body["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		return fmt.Errorf("request missing messages array")
	}

	var sawSystem bool
	var sawUser bool
	for _, raw := range rawMessages {
		msg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := stringValue(msg["role"])
		switch role {
		case "system", "developer":
			if containsTextFragment(msg["content"], wantSystem) {
				sawSystem = true
			}
		case "user":
			if containsTextFragment(msg["content"], wantPrompt) {
				sawUser = true
			}
		}
	}

	if !sawSystem {
		return fmt.Errorf("request missing system/developer instruction %q", wantSystem)
	}
	if !sawUser {
		return fmt.Errorf("request missing user prompt %q", wantPrompt)
	}
	return nil
}

func assertResponsesRequest(body map[string]any, wantSystem, wantPrompt string) error {
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		return fmt.Errorf("request missing model")
	}

	sawSystem := containsTextFragment(body["instructions"], wantSystem) ||
		rolePayloadContainsText(body["input"], []string{"system", "developer"}, wantSystem)
	sawPrompt := containsTextFragment(body["input"], wantPrompt) ||
		rolePayloadContainsText(body["input"], []string{"user"}, wantPrompt)

	if !sawSystem {
		return fmt.Errorf("request missing system/developer instruction %q", wantSystem)
	}
	if !sawPrompt {
		return fmt.Errorf("request missing user input %q", wantPrompt)
	}
	return nil
}

func assertAnthropicMessagesRequest(body map[string]any, wantSystem, wantPrompt string, wantMaxTokens int) error {
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		return fmt.Errorf("request missing model")
	}
	if intValue(body["max_tokens"]) != wantMaxTokens {
		return fmt.Errorf("request max_tokens=%d, want %d", intValue(body["max_tokens"]), wantMaxTokens)
	}
	if !containsTextFragment(body["system"], wantSystem) {
		return fmt.Errorf("request missing system prompt %q", wantSystem)
	}

	rawMessages, ok := body["messages"].([]any)
	if !ok || len(rawMessages) != 1 {
		return fmt.Errorf("request missing user messages array")
	}
	msg, ok := rawMessages[0].(map[string]any)
	if !ok {
		return fmt.Errorf("request message has unexpected shape")
	}
	if stringValue(msg["role"]) != "user" {
		return fmt.Errorf("request role=%q, want user", stringValue(msg["role"]))
	}
	if !containsTextFragment(msg["content"], wantPrompt) {
		return fmt.Errorf("request missing user prompt %q", wantPrompt)
	}
	return nil
}

func rolePayloadContainsText(v any, roles []string, want string) bool {
	switch typed := v.(type) {
	case []any:
		for _, item := range typed {
			if rolePayloadContainsText(item, roles, want) {
				return true
			}
		}
	case map[string]any:
		role := stringValue(typed["role"])
		for _, allowed := range roles {
			if role == allowed && containsTextFragment(typed["content"], want) {
				return true
			}
		}
		for _, item := range typed {
			if rolePayloadContainsText(item, roles, want) {
				return true
			}
		}
	}
	return false
}

func containsTextFragment(v any, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, text := range collectTextFragments(v) {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func collectTextFragments(v any) []string {
	switch typed := v.(type) {
	case string:
		return []string{typed}
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, collectTextFragments(item)...)
		}
		return out
	case map[string]any:
		var out []string
		for _, value := range typed {
			out = append(out, collectTextFragments(value)...)
		}
		return out
	default:
		return nil
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func intValue(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}
