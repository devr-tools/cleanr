package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

// The PostHog and Langfuse sinks send credentials outside applyAuth (request
// body / Basic auth), so they must enforce the same egress policy: a
// provider secret may not be routed to an arbitrary config-controlled host.
func TestNativeSinksRefuseProviderSecretToUntrustedHost(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-not-real")

	cases := []struct {
		name string
		sink cleanr.ResultSinkConfig
	}{
		{
			name: "posthog project token",
			sink: cleanr.ResultSinkConfig{
				Name:            "posthog",
				Type:            "posthog",
				BaseURL:         "https://evil.example.com",
				ProjectTokenEnv: "OPENAI_API_KEY",
			},
		},
		{
			name: "langfuse secret key",
			sink: cleanr.ResultSinkConfig{
				Name:         "langfuse",
				Type:         "langfuse",
				BaseURL:      "https://evil.example.com",
				PublicKeyEnv: "OPENAI_API_KEY",
				SecretKeyEnv: "OPENAI_API_KEY",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := cleanr.PublishResultSinks(
				context.Background(),
				cleanr.IntegrationsConfig{ResultSinks: []cleanr.ResultSinkConfig{tc.sink}},
				cleanr.Report{Name: "demo"},
				nil,
				nil,
			)
			if len(results) != 1 {
				t.Fatalf("expected one sink result, got %d", len(results))
			}
			if results[0].Published {
				t.Fatalf("expected publish to be refused: %+v", results[0])
			}
			if !strings.Contains(results[0].Message, "refusing to send credential") {
				t.Fatalf("expected egress refusal message, got %q", results[0].Message)
			}
		})
	}
}
