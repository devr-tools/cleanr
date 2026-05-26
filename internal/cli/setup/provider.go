package setup

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
)

func resolveAgentProvider(prompter setupPrompter, override profilepkg.Provider, ciMode, browserMode bool) (profilepkg.Provider, error) {
	if ciMode {
		return ciProviderProfile(override)
	}

	if hasProviderOverride(override) {
		provider, err := gatherProviderProfile(prompter, override, false, browserMode)
		if err != nil {
			return profilepkg.Provider{}, err
		}
		if err := profilepkg.UpsertProvider(provider); err != nil {
			return profilepkg.Provider{}, fmt.Errorf("save profile: %w", err)
		}
		return provider, nil
	}

	provider, err := profilepkg.DefaultProvider()
	if err == nil {
		return provider, nil
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no default provider") {
		return profilepkg.Provider{}, err
	}

	provider, err = gatherProviderProfile(prompter, override, false, browserMode)
	if err != nil {
		return profilepkg.Provider{}, err
	}
	if err := profilepkg.UpsertProvider(provider); err != nil {
		return profilepkg.Provider{}, fmt.Errorf("save profile: %w", err)
	}
	return provider, nil
}

func gatherProviderProfile(prompter setupPrompter, initial profilepkg.Provider, ciMode, browserMode bool) (profilepkg.Provider, error) {
	if ciMode {
		return ciProviderProfile(initial)
	}

	providerName, err := prompter.askChoice("Provider", firstNonEmpty(initial.Name, "openai"), []string{"openai", "anthropic"})
	if err != nil {
		return profilepkg.Provider{}, err
	}

	provider := profilepkg.Provider{
		Name:      providerName,
		Model:     strings.TrimSpace(initial.Model),
		APIKey:    strings.TrimSpace(initial.APIKey),
		APIKeyEnv: strings.TrimSpace(initial.APIKeyEnv),
		APIMode:   strings.TrimSpace(initial.APIMode),
		BaseURL:   strings.TrimSpace(initial.BaseURL),
		MaxTokens: initial.MaxTokens,
	}

	if err := prompter.confirmOpenBrowser(displayProviderName(provider.Name), providerAuthURL(provider.Name), browserMode); err != nil {
		return profilepkg.Provider{}, err
	}

	switch provider.Name {
	case "openai":
		provider.APIMode, err = prompter.askChoice("OpenAI API mode", firstNonEmpty(provider.APIMode, defaultOpenAIAPIMode), []string{"responses", "chat_completions"})
		if err != nil {
			return profilepkg.Provider{}, err
		}
		provider.Model, err = prompter.ask("OpenAI model", firstNonEmpty(provider.Model, defaultOpenAIModel))
		if err != nil {
			return profilepkg.Provider{}, err
		}
		provider.APIKeyEnv, err = prompter.ask("OpenAI API key env", firstNonEmpty(provider.APIKeyEnv, defaultOpenAIKeyEnv))
		if err != nil {
			return profilepkg.Provider{}, err
		}
		provider.APIKey, err = prompter.askSecret("OpenAI API key", provider.APIKey)
		if err != nil {
			return profilepkg.Provider{}, err
		}
		if strings.TrimSpace(provider.APIKey) == "" {
			return profilepkg.Provider{}, fmt.Errorf("openai api key is required")
		}
	case "anthropic":
		provider.Model, err = prompter.ask("Anthropic model", firstNonEmpty(provider.Model, defaultAnthropicModel))
		if err != nil {
			return profilepkg.Provider{}, err
		}
		provider.APIKeyEnv, err = prompter.ask("Anthropic API key env", firstNonEmpty(provider.APIKeyEnv, defaultAnthropicKeyEnv))
		if err != nil {
			return profilepkg.Provider{}, err
		}
		provider.APIKey, err = prompter.askSecret("Anthropic API key", provider.APIKey)
		if err != nil {
			return profilepkg.Provider{}, err
		}
		if strings.TrimSpace(provider.APIKey) == "" {
			return profilepkg.Provider{}, fmt.Errorf("anthropic api key is required")
		}
		maxTokens := provider.MaxTokens
		if maxTokens <= 0 {
			maxTokens = defaultAnthropicMaxTokens
		}
		maxTokensRaw, err := prompter.ask("Anthropic max tokens", fmt.Sprintf("%d", maxTokens))
		if err != nil {
			return profilepkg.Provider{}, err
		}
		if _, err := fmt.Sscanf(strings.TrimSpace(maxTokensRaw), "%d", &provider.MaxTokens); err != nil || provider.MaxTokens <= 0 {
			return profilepkg.Provider{}, fmt.Errorf("anthropic max tokens must be a positive integer")
		}
	default:
		return profilepkg.Provider{}, fmt.Errorf("provider must be one of openai or anthropic")
	}

	return provider, nil
}

func gatherAgentInputs(prompter setupPrompter, nameFlag, systemPromptFlag, userPromptFlag string, ciMode bool) (string, string, string, error) {
	if ciMode {
		agentName := firstNonEmpty(nameFlag, os.Getenv("CLEANR_AGENT_NAME"), defaultAgentName)
		systemPrompt := firstNonEmpty(systemPromptFlag, os.Getenv("CLEANR_SYSTEM_PROMPT"))
		if strings.TrimSpace(systemPrompt) == "" {
			return "", "", "", fmt.Errorf("system prompt is required in CI mode; set -system-prompt or CLEANR_SYSTEM_PROMPT")
		}
		userPrompt := firstNonEmpty(userPromptFlag, os.Getenv("CLEANR_USER_PROMPT"), defaultAgentPrompt)
		return agentName, systemPrompt, userPrompt, nil
	}

	agentName, err := prompter.ask("Agent name", firstNonEmpty(nameFlag, defaultAgentName))
	if err != nil {
		return "", "", "", err
	}
	systemPrompt, err := prompter.askRequired("System prompt", systemPromptFlag)
	if err != nil {
		return "", "", "", err
	}
	userPrompt, err := prompter.ask("Primary user prompt", firstNonEmpty(userPromptFlag, defaultAgentPrompt))
	if err != nil {
		return "", "", "", err
	}
	return agentName, systemPrompt, userPrompt, nil
}

func ciProviderProfile(initial profilepkg.Provider) (profilepkg.Provider, error) {
	providerName := strings.ToLower(strings.TrimSpace(firstNonEmpty(initial.Name, os.Getenv("CLEANR_PROVIDER"), "openai")))
	switch providerName {
	case "openai":
		provider := initializedProvider(profilepkg.Provider{
			Name:      "openai",
			Model:     firstNonEmpty(initial.Model, os.Getenv("CLEANR_MODEL"), defaultOpenAIModel),
			APIMode:   normalizedOpenAIAPIMode(firstNonEmpty(initial.APIMode, os.Getenv("CLEANR_OPENAI_API_MODE"), defaultOpenAIAPIMode)),
			APIKeyEnv: firstNonEmpty(initial.APIKeyEnv, os.Getenv("CLEANR_API_KEY_ENV"), defaultOpenAIKeyEnv),
			APIKey:    firstNonEmpty(initial.APIKey, os.Getenv("CLEANR_API_KEY")),
			BaseURL:   firstNonEmpty(initial.BaseURL, os.Getenv("CLEANR_BASE_URL")),
		})
		if err := validateOpenAIProfile(provider); err != nil {
			return profilepkg.Provider{}, err
		}
		return provider, nil
	case "anthropic":
		maxTokens, err := resolveAnthropicMaxTokens(initial.MaxTokens)
		if err != nil {
			return profilepkg.Provider{}, err
		}
		return initializedProvider(profilepkg.Provider{
			Name:      "anthropic",
			Model:     firstNonEmpty(initial.Model, os.Getenv("CLEANR_MODEL"), defaultAnthropicModel),
			APIKeyEnv: firstNonEmpty(initial.APIKeyEnv, os.Getenv("CLEANR_API_KEY_ENV"), defaultAnthropicKeyEnv),
			APIKey:    firstNonEmpty(initial.APIKey, os.Getenv("CLEANR_API_KEY")),
			BaseURL:   firstNonEmpty(initial.BaseURL, os.Getenv("CLEANR_BASE_URL")),
			MaxTokens: maxTokens,
		}), nil
	default:
		return profilepkg.Provider{}, fmt.Errorf("provider must be one of openai or anthropic")
	}
}

func resolveAnthropicMaxTokens(flagValue int) (int, error) {
	if flagValue > 0 {
		return flagValue, nil
	}
	raw := strings.TrimSpace(os.Getenv("CLEANR_ANTHROPIC_MAX_TOKENS"))
	if raw == "" {
		return defaultAnthropicMaxTokens, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("CLEANR_ANTHROPIC_MAX_TOKENS must be a positive integer")
	}
	return value, nil
}

func validateOpenAIProfile(provider profilepkg.Provider) error {
	switch provider.APIMode {
	case "responses", "chat_completions":
		return nil
	default:
		return fmt.Errorf("openai api mode must be responses or chat_completions")
	}
}

func initializedProvider(provider profilepkg.Provider) profilepkg.Provider {
	provider.Name = strings.ToLower(strings.TrimSpace(provider.Name))
	provider.Model = strings.TrimSpace(provider.Model)
	provider.APIMode = strings.TrimSpace(provider.APIMode)
	provider.APIKeyEnv = strings.TrimSpace(provider.APIKeyEnv)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	return provider
}

func hasProviderOverride(provider profilepkg.Provider) bool {
	return strings.TrimSpace(provider.Name) != "" ||
		strings.TrimSpace(provider.Model) != "" ||
		strings.TrimSpace(provider.APIKey) != "" ||
		strings.TrimSpace(provider.APIKeyEnv) != "" ||
		strings.TrimSpace(provider.APIMode) != "" ||
		strings.TrimSpace(provider.BaseURL) != "" ||
		provider.MaxTokens > 0
}

func providerAuthURL(providerName string) string {
	switch providerName {
	case "anthropic":
		return "https://console.anthropic.com/settings/keys"
	default:
		return "https://platform.openai.com/settings/organization/api-keys"
	}
}

func displayProviderName(providerName string) string {
	switch providerName {
	case "anthropic":
		return "Anthropic"
	default:
		return "OpenAI"
	}
}

func normalizedOpenAIAPIMode(value string) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" {
		return defaultOpenAIAPIMode
	}
	return mode
}
