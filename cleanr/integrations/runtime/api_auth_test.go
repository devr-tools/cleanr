package runtime

import (
	"net/http"
	"testing"
)

func TestApplyAuthRefusesProviderSecretToUntrustedHost(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret")
	h := http.Header{}
	applyAuth(h, "OPENAI_API_KEY", "https://evil.example.com/collect")
	if got := h.Get("Authorization"); got != "" {
		t.Fatalf("expected provider secret to be withheld from untrusted host, got %q", got)
	}
}

func TestApplyAuthAllowsProviderSecretToAllowlistedHost(t *testing.T) {
	t.Setenv("BRAINTRUST_API_KEY", "bt-secret")
	h := http.Header{}
	applyAuth(h, "BRAINTRUST_API_KEY", "https://api.braintrust.dev/v1/experiment")
	if got := h.Get("Authorization"); got != "Bearer bt-secret" {
		t.Fatalf("expected credential to be sent to allowlisted host, got %q", got)
	}
}

func TestApplyAuthAllowsProviderSecretToLoopback(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret")
	h := http.Header{}
	applyAuth(h, "OPENAI_API_KEY", "http://127.0.0.1:8080/ingest")
	if got := h.Get("Authorization"); got != "Bearer sk-secret" {
		t.Fatalf("expected credential to reach loopback, got %q", got)
	}
}

func TestApplyAuthAllowsGenericTokenAnywhere(t *testing.T) {
	t.Setenv("MY_INGEST_TOKEN", "tok")
	h := http.Header{}
	applyAuth(h, "MY_INGEST_TOKEN", "https://sink.example.com/publish")
	if got := h.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("expected non-provider token to be sent, got %q", got)
	}
}
