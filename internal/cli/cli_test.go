package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"cleanr/cleanr"
)

func TestValidateCmdPrintsActionableFieldErrors(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Target.URL = ""
	cfg.Scenarios[0].Input = ""
	cfg.Reporting.Format = "markdown"

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	path := t.TempDir() + "/cleanr.json"
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := validateCmd([]string{"-config", path}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	output := stderr.String()
	for _, want := range []string{
		"invalid: invalid config:",
		"target.url: is required. Fix: set target.url to the full API endpoint URL",
		"scenarios[0].input: is required. Fix: set the end-user prompt or test input for this scenario",
		"reporting.format: must be one of text, json, or junit",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output:\n%s", want, output)
		}
	}
}
