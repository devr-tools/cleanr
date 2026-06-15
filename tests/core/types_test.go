package tests

import (
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestCoreHelperAccessorsAndFacadeMarshaling(t *testing.T) {
	t.Parallel()

	targetCfg := cleanr.TargetConfig{TimeoutMS: 1500}
	if targetCfg.Timeout().Milliseconds() != 1500 {
		t.Fatalf("unexpected timeout: %s", targetCfg.Timeout())
	}
	if targetCfg.TargetType() != "http" {
		t.Fatalf("expected default http target type")
	}
	if (cleanr.TargetConfig{Type: " OpenAI "}).TargetType() != "openai" {
		t.Fatalf("expected normalized openai target type")
	}
	if (cleanr.OpenAIConfig{}).ProviderValue("openai_compatible") != "openai_compatible" || (cleanr.OpenAIConfig{Provider: " Ollama "}).ProviderValue("openai_compatible") != "ollama" {
		t.Fatal("unexpected openai provider normalization")
	}
	if (cleanr.OpenAIConfig{}).AuthHeaderValue() != "Authorization" || (cleanr.OpenAIConfig{AuthHeader: " api-key "}).AuthHeaderValue() != "api-key" {
		t.Fatal("unexpected openai auth header normalization")
	}
	if (cleanr.OpenAIConfig{}).AuthSchemeValue() != "Bearer" || (cleanr.OpenAIConfig{AuthScheme: "Token"}).AuthSchemeValue() != "Token" {
		t.Fatal("unexpected openai auth scheme normalization")
	}
	if (cleanr.OpenAIConfig{}).APIModeValue() != "responses" || (cleanr.OpenAIConfig{APIMode: " CHAT_COMPLETIONS "}).APIModeValue() != "chat_completions" {
		t.Fatal("unexpected openai api mode normalization")
	}
	if (cleanr.AnthropicConfig{}).VersionValue() != "2023-06-01" || (cleanr.AnthropicConfig{Version: " 2025-01-01 "}).VersionValue() != "2025-01-01" {
		t.Fatal("unexpected anthropic version normalization")
	}
	if (cleanr.AnthropicConfig{}).MaxTokensValue() != 1024 || (cleanr.AnthropicConfig{MaxTokens: 2048}).MaxTokensValue() != 2048 {
		t.Fatal("unexpected anthropic max_tokens normalization")
	}
	scenario := cleanr.Scenario{
		Name: "multi-turn",
		Turns: []cleanr.ConversationTurn{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "First request"},
			{Role: "assistant", Content: "First answer"},
			{Role: "user", Content: "Second request"},
		},
	}
	if scenario.SystemValue() != "You are helpful." || scenario.InputValue() != "Second request" {
		t.Fatalf("unexpected transcript accessors: %+v", scenario)
	}
	req := cleanr.BuildScenarioRequest(scenario, targetCfg.Timeout())
	if len(req.Messages) != 4 || req.System != "You are helpful." || req.Prompt != "Second request" {
		t.Fatalf("unexpected scenario request: %+v", req)
	}

	cfg := cleanr.ExampleConfig()
	jsonData, err := cleanr.MarshalConfig(cfg, "json")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if len(jsonData) == 0 || jsonData[len(jsonData)-1] != '\n' {
		t.Fatalf("expected newline-terminated json output: %q", string(jsonData))
	}

	yamlData, err := cleanr.MarshalConfig(cfg, "yaml")
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if len(yamlData) == 0 || yamlData[len(yamlData)-1] != '\n' {
		t.Fatalf("expected newline-terminated yaml output: %q", string(yamlData))
	}
}
