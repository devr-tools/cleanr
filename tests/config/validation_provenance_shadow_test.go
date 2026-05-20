package tests

import (
	"strings"
	"testing"

	"cleanr/cleanr"
)

func TestValidateConfigRejectsInvalidContextSource(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "invalid-source",
		Input: "hello",
		ContextSources: []cleanr.ContextSource{{
			Kind:    "retrieval",
			Trust:   "maybe",
			Content: "source text",
		}},
	}}

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "context_sources[0].kind") || !strings.Contains(msg, "context_sources[0].trust") {
		t.Fatalf("expected context source validation errors, got %s", msg)
	}
}

func TestValidateConfigRejectsShadowStateWithoutRoots(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{Name: "shadow", Input: "hello"}}
	cfg.Suites.PromptInjection.Enabled = false
	cfg.Suites.Security.Enabled = false
	cfg.Suites.Load.Enabled = false
	cfg.Suites.Chaos.Enabled = false
	cfg.Suites.Drift.Enabled = false
	cfg.Suites.Provenance.Enabled = false
	cfg.Suites.TokenOptimization.Enabled = false
	cfg.Suites.ShadowState.Enabled = true
	cfg.Suites.ShadowState.Roots = nil

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "suites.shadow_state.roots") {
		t.Fatalf("expected shadow_state roots validation error, got %s", err.Error())
	}
}

func TestValidateConfigRejectsInvalidExpectedMutation(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "shadow",
		Input: "hello",
		ExpectedMutations: []cleanr.ExpectedMutation{{
			Path:            "tmp/out.txt",
			Kind:            "rename",
			ContentContains: "hello",
		}},
	}}

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "expected_mutations[0].kind") {
		t.Fatalf("expected expected_mutations kind validation error, got %s", err.Error())
	}
}

func TestValidateConfigRejectsDeletedExpectedMutationWithContentAssertion(t *testing.T) {
	cfg := cleanr.ExampleConfig()
	cfg.Scenarios = []cleanr.Scenario{{
		Name:  "shadow",
		Input: "hello",
		ExpectedMutations: []cleanr.ExpectedMutation{{
			Path:            "tmp/out.txt",
			Kind:            "deleted",
			ContentContains: "hello",
		}},
	}}

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "expected_mutations[0].content_contains") {
		t.Fatalf("expected deleted expected_mutation content validation error, got %s", err.Error())
	}
}
