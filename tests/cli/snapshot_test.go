package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
	"github.com/devr-tools/cleanr/internal/cli"
)

type snapshotRoundTripper func(*http.Request) (*http.Response, error)

func (f snapshotRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func snapshotJSONResponse(t *testing.T, statusCode int, body map[string]any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

func TestCLISnapshotCommandWritesBaseline(t *testing.T) {
	// Not parallel: this test swaps the process-global http.DefaultTransport,
	// which parallel tests read through clients with a nil Transport. Sequential
	// tests never overlap parallel ones, so the swap is race-free only without
	// t.Parallel().
	original := http.DefaultTransport
	http.DefaultTransport = snapshotRoundTripper(func(req *http.Request) (*http.Response, error) {
		return snapshotJSONResponse(t, http.StatusOK, map[string]any{
			"output": map[string]any{"text": "baseline answer"},
		}), nil
	})
	defer func() { http.DefaultTransport = original }()

	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{Name: "stable", Input: "hello"}}
	cfg.Suites.Drift.StableTags = []string{"stable"}
	cfg.Suites.Drift.BaselineFile = "nested/baseline.yaml"
	path := filepath.Join(t.TempDir(), "cleanr.json")
	if err := cleanr.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run([]string{"snapshot", "-config", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected snapshot command to succeed, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote 1 snapshots to") {
		t.Fatalf("unexpected snapshot stdout: %s", stdout.String())
	}
}
