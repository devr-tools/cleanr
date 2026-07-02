package tests

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

// When a 429's Retry-After exceeds the remaining context budget, the response
// is returned instead of retried — and its body must still be readable. It
// used to be closed before being handed back, so callers saw "read on closed
// response body" instead of the actual rate-limit response.
func TestHTTPTargetReturnsReadableBodyWhenRetryBudgetExhausted(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		resp := jsonResponse(t, http.StatusTooManyRequests, map[string]any{
			"error": map[string]any{"message": "rate limited"},
		})
		resp.Header.Set("Retry-After", "3600")
		return resp, nil
	})}

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		Type:   "http",
		URL:    "https://api.test/v1",
		Method: http.MethodPost,
	}, client)

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "rate-limited",
		Input: "hello",
	}, time.Second))

	if resp.Err != nil {
		t.Fatalf("expected readable rate-limit response, got error: %v", resp.Err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 status, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(resp.Body), "rate limited") {
		t.Fatalf("expected rate-limit body to be readable, got %q", string(resp.Body))
	}
}

// A transport error on a POST must not be retried: the connection can die
// after the request was fully delivered, and replaying it risks duplicate
// writes or charges.
func TestHTTPTargetDoesNotRetryPOSTOnTransportError(t *testing.T) {
	t.Parallel()

	attempts := 0
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		return nil, errors.New("connection reset by peer")
	})}

	target := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		Type:   "http",
		URL:    "https://api.test/v1",
		Method: http.MethodPost,
	}, client)

	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "post-reset",
		Input: "hello",
	}, time.Second))

	if resp.Err == nil {
		t.Fatal("expected transport error to surface")
	}
	if attempts != 1 {
		t.Fatalf("POST must not be replayed after a transport error, got %d attempts", attempts)
	}
}

// Idempotent requests keep retrying transport errors, and 429/503 responses
// keep retrying for every method (the server rejected the request, so a
// replay cannot double-apply it).
func TestHTTPTargetRetriesIdempotentTransportErrorsAndRetryableStatuses(t *testing.T) {
	t.Parallel()

	getAttempts := 0
	getClient := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		getAttempts++
		if getAttempts < 3 {
			return nil, errors.New("connection refused")
		}
		return jsonResponse(t, http.StatusOK, map[string]any{"output": "recovered"}), nil
	})}
	getTarget := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		Type:   "http",
		URL:    "https://api.test/v1",
		Method: http.MethodGet,
	}, getClient)
	resp := getTarget.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "get-flaky",
		Input: "hello",
	}, 10*time.Second))
	if resp.Err != nil {
		t.Fatalf("expected GET to retry transport errors and succeed, got %v", resp.Err)
	}
	if getAttempts != 3 {
		t.Fatalf("expected 3 GET attempts, got %d", getAttempts)
	}

	postAttempts := 0
	postClient := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		postAttempts++
		if postAttempts < 2 {
			resp := jsonResponse(t, http.StatusServiceUnavailable, map[string]any{"error": "overloaded"})
			resp.Header.Set("Retry-After", "0")
			return resp, nil
		}
		return jsonResponse(t, http.StatusOK, map[string]any{"output": "recovered"}), nil
	})}
	postTarget := cleanr.NewHTTPTarget(cleanr.TargetConfig{
		Type:   "http",
		URL:    "https://api.test/v1",
		Method: http.MethodPost,
	}, postClient)
	resp = postTarget.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "post-503",
		Input: "hello",
	}, 10*time.Second))
	if resp.Err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected POST to retry 503 and succeed, got err=%v status=%d", resp.Err, resp.StatusCode)
	}
	if postAttempts != 2 {
		t.Fatalf("expected 2 POST attempts on 503, got %d", postAttempts)
	}
}
