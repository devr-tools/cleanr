package tests

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestCLITargetInvokeCapturesStdoutAndExtractsJSON(t *testing.T) {
	t.Parallel()

	target := cleanr.NewCLITarget(cleanr.TargetConfig{
		Type:          "cli",
		ResponseField: "output.text",
		CLI: cleanr.CLIConfig{
			Command: os.Args[0],
			Args:    []string{"-test.run=TestCLIAdapterHelperProcess", "--", "success"},
		},
	})

	// A generous timeout: the helper is this test binary re-exec'd, which under
	// -race instrumentation can take well over a second to start.
	resp := target.Invoke(context.Background(), cleanr.BuildScenarioRequest(cleanr.Scenario{
		Name:  "cli-target",
		Input: "hello",
	}, 10*time.Second))
	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected cli response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Text != "hello from cli" {
		t.Fatalf("unexpected cli text: %q", resp.Text)
	}
	if resp.ExitCode != 0 || resp.Normalized.Provider != "cli" {
		t.Fatalf("unexpected cli response: %+v", resp)
	}
}

func TestCLITargetInvokeCapturesExitCodeAndStderr(t *testing.T) {
	t.Parallel()

	target := cleanr.NewCLITarget(cleanr.TargetConfig{
		Type: "cli",
		CLI: cleanr.CLIConfig{
			Command: os.Args[0],
			Args:    []string{"-test.run=TestCLIAdapterHelperProcess", "--", "fail"},
		},
	})

	resp := target.Invoke(context.Background(), cleanr.Request{Timeout: 10 * time.Second})
	if resp.Err == nil {
		t.Fatal("expected cli error")
	}
	if resp.ExitCode != 3 {
		t.Fatalf("unexpected cli exit code: %+v", resp)
	}
	if strings.TrimSpace(resp.Stderr) != "cli failed loudly" {
		t.Fatalf("unexpected cli stderr: %q", resp.Stderr)
	}
}

func TestCLIAdapterHelperProcess(t *testing.T) {
	mode := helperProcessMode(os.Args)
	if mode == "" {
		return
	}

	var req cleanr.Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		t.Fatalf("decode helper stdin: %v", err)
	}
	switch mode {
	case "success":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"output": map[string]any{
				"text": "hello from cli",
				"echo": req.Prompt,
			},
		})
		os.Exit(0)
	case "fail":
		_, _ = os.Stderr.WriteString("cli failed loudly\n")
		os.Exit(3)
	}
}

func helperProcessMode(args []string) string {
	for i := range args {
		if args[i] == "--" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
