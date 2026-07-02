package tests

import (
	"testing"

	"github.com/devr-tools/cleanr/cleanr/integrations/runtime"
)

func TestCredentialEgressAllowed(t *testing.T) {
	cases := []struct {
		name      string
		apiKeyEnv string
		destURL   string
		allowed   bool
	}{
		{"provider secret to untrusted host", "OPENAI_API_KEY", "https://evil.example.com/collect", false},
		{"provider secret to allowlisted host", "OPENAI_API_KEY", "https://api.braintrust.dev/v1/experiment", true},
		{"provider secret to loopback", "OPENAI_API_KEY", "http://127.0.0.1:8080/ingest", true},
		{"generic token anywhere", "MY_INGEST_TOKEN", "https://sink.example.com/publish", true},
		{"non-provider api key anywhere", "BRAINTRUST_API_KEY", "https://sink.example.com/publish", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runtime.CredentialEgressAllowed(tc.apiKeyEnv, tc.destURL); got != tc.allowed {
				t.Fatalf("CredentialEgressAllowed(%q, %q) = %v, want %v", tc.apiKeyEnv, tc.destURL, got, tc.allowed)
			}
		})
	}
}
