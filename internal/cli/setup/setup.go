package setup

import (
	"flag"
	"fmt"
	"io"

	profilepkg "github.com/devr-tools/cleanr/cleanr/profile"
)

const (
	defaultConfigPath      = "cleanr.yaml"
	defaultAgentConfigPath = "cleanr.agent.yaml"
	defaultAgentName       = "agent-under-test"
	defaultAgentPrompt     = "Help a customer reset their password and confirm the next step."

	defaultOpenAIModel        = "gpt-4.1-mini"
	defaultOpenAIAPIMode      = "responses"
	defaultOpenAIKeyEnv       = "OPENAI_API_KEY"
	defaultAnthropicModel     = "claude-sonnet-4-20250514"
	defaultAnthropicKeyEnv    = "ANTHROPIC_API_KEY"
	defaultAnthropicMaxTokens = 1024
	defaultBraintrustKeyEnv   = "BRAINTRUST_API_KEY"
	defaultLangfusePublicEnv  = "LANGFUSE_PUBLIC_KEY"
	defaultLangfuseSecretEnv  = "LANGFUSE_SECRET_KEY"
	defaultPostHogTokenEnv    = "POSTHOG_PROJECT_TOKEN"
	defaultWebhookTokenEnv    = "CLEANR_RESULTS_WEBHOOK_TOKEN"
	defaultAttestationKeyEnv  = "CLEANR_ATTESTATION_KEY"
	defaultAttestationKeyID   = "ci-ed25519"
	defaultIntegrationFamily  = "cleanr-ci"
	profilePR                 = "pr"
	profileMain               = "main"
	profileRelease            = "release"
)

type starterConfigOptions struct {
	Profile              string
	TrendGatePreset      string
	WithBraintrust       bool
	BraintrustProject    string
	BraintrustExperiment string
	BraintrustAPIKeyEnv  string
	BraintrustBaseURL    string
	WithLangfuse         bool
	LangfusePublicKeyEnv string
	LangfuseSecretKeyEnv string
	LangfuseBaseURL      string
	LangfuseExperiment   string
	WithPostHog          bool
	PostHogTokenEnv      string
	PostHogBaseURL       string
	PostHogExperiment    string
	WithWebhook          bool
	WebhookEndpoint      string
	WebhookAPIKeyEnv     string
	WithAttestation      bool
	AttestationKeyEnv    string
	AttestationKeyID     string
}

type setupPrompter interface {
	ask(label, fallback string) (string, error)
	askRequired(label, fallback string) (string, error)
	askChoice(label, fallback string, options []string) (string, error)
	askSecret(label, fallback string) (string, error)
	confirmOpenBrowser(providerName, url string, force bool) error
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "agent" {
		return setupAgentCmd(args[1:], stdin, stdout, stderr)
	}
	return setupProviderCmd(args, stdin, stdout, stderr)
}

func setupProviderCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", defaultConfigPath, "Path to write the generated cleanr config")
	force := fs.Bool("force", false, "Overwrite an existing output file")
	providerFlag := fs.String("provider", "", "Provider to configure: openai or anthropic")
	modelFlag := fs.String("model", "", "Provider model name")
	apiKeyFlag := fs.String("api-key", "", "Provider API key to store locally")
	apiKeyEnvFlag := fs.String("api-key-env", "", "Environment variable name used by the generated config")
	apiModeFlag := fs.String("api-mode", "", "OpenAI API mode: responses or chat_completions")
	baseURLFlag := fs.String("base-url", "", "Optional provider base URL override")
	maxTokensFlag := fs.Int("max-tokens", 0, "Optional Anthropic max_tokens override")
	profileFlag := fs.String("profile", "", "Starter profile: pr, main, or release")
	trendGatePreset := fs.String("trend-gate-preset", "", "Optional trend gate preset override: strict, moderate, or exploratory")
	withBraintrust := fs.Bool("with-braintrust", false, "Include Braintrust result publishing and remote trend comparison using standard env-based secrets")
	braintrustProject := fs.String("braintrust-project", "", "Braintrust project name for native result publishing")
	braintrustExperiment := fs.String("braintrust-experiment", defaultIntegrationFamily, "Braintrust experiment family name")
	braintrustAPIKeyEnv := fs.String("braintrust-api-key-env", defaultBraintrustKeyEnv, "Environment variable name used for the Braintrust API key")
	braintrustBaseURL := fs.String("braintrust-base-url", "", "Optional Braintrust base URL override")
	withLangfuse := fs.Bool("with-langfuse", false, "Include Langfuse result publishing using standard env-based secrets")
	langfusePublicKeyEnv := fs.String("langfuse-public-key-env", defaultLangfusePublicEnv, "Environment variable name used for the Langfuse public key")
	langfuseSecretKeyEnv := fs.String("langfuse-secret-key-env", defaultLangfuseSecretEnv, "Environment variable name used for the Langfuse secret key")
	langfuseBaseURL := fs.String("langfuse-base-url", "", "Optional Langfuse base URL override")
	langfuseExperiment := fs.String("langfuse-experiment", defaultIntegrationFamily, "Langfuse trace family name")
	withPostHog := fs.Bool("with-posthog", false, "Include PostHog result publishing using standard env-based secrets")
	posthogTokenEnv := fs.String("posthog-project-token-env", defaultPostHogTokenEnv, "Environment variable name used for the PostHog project token")
	posthogBaseURL := fs.String("posthog-base-url", "", "Optional PostHog base URL override")
	posthogExperiment := fs.String("posthog-experiment", defaultIntegrationFamily, "PostHog event family name")
	withWebhook := fs.Bool("with-webhook", false, "Include generic webhook result publishing")
	webhookEndpoint := fs.String("webhook-endpoint", "", "Webhook endpoint URL for generic result publishing")
	webhookAPIKeyEnv := fs.String("webhook-api-key-env", defaultWebhookTokenEnv, "Environment variable name used for the optional webhook bearer token")
	withAttestation := fs.Bool("with-attestation", false, "Enable signed release-gate attestations using a key from an environment variable")
	attestationKeyEnv := fs.String("attestation-key-env", defaultAttestationKeyEnv, "Environment variable name used for the attestation signing key")
	attestationKeyID := fs.String("attestation-key-id", defaultAttestationKeyID, "Stable key identifier embedded in generated attestations")
	ciMode := fs.Bool("ci", false, "Generate config non-interactively for CI and do not store credentials locally")
	browserMode := fs.Bool("browser", false, "Open the provider browser dashboard automatically during interactive setup")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := ensureWritableOutput(*output, *force); err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	providerOverride := profilepkg.Provider{
		Name:      *providerFlag,
		Model:     *modelFlag,
		APIKey:    *apiKeyFlag,
		APIKeyEnv: *apiKeyEnvFlag,
		APIMode:   *apiModeFlag,
		BaseURL:   *baseURLFlag,
		MaxTokens: *maxTokensFlag,
	}

	options, err := resolveStarterConfigOptions(starterConfigOptions{
		Profile:              *profileFlag,
		TrendGatePreset:      *trendGatePreset,
		WithBraintrust:       *withBraintrust,
		BraintrustProject:    *braintrustProject,
		BraintrustExperiment: *braintrustExperiment,
		BraintrustAPIKeyEnv:  *braintrustAPIKeyEnv,
		BraintrustBaseURL:    *braintrustBaseURL,
		WithLangfuse:         *withLangfuse,
		LangfusePublicKeyEnv: *langfusePublicKeyEnv,
		LangfuseSecretKeyEnv: *langfuseSecretKeyEnv,
		LangfuseBaseURL:      *langfuseBaseURL,
		LangfuseExperiment:   *langfuseExperiment,
		WithPostHog:          *withPostHog,
		PostHogTokenEnv:      *posthogTokenEnv,
		PostHogBaseURL:       *posthogBaseURL,
		PostHogExperiment:    *posthogExperiment,
		WithWebhook:          *withWebhook,
		WebhookEndpoint:      *webhookEndpoint,
		WebhookAPIKeyEnv:     *webhookAPIKeyEnv,
		WithAttestation:      *withAttestation,
		AttestationKeyEnv:    *attestationKeyEnv,
		AttestationKeyID:     *attestationKeyID,
	}, *ciMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	prompter := newSetupPrompter(stdin, stdout, *ciMode)
	provider, err := gatherProviderProfile(prompter, providerOverride, *ciMode, *browserMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	if !*ciMode {
		if err := profilepkg.UpsertProvider(provider); err != nil {
			_, _ = fmt.Fprintf(stderr, "setup error: save profile: %v\n", err)
			return 2
		}
	}

	if err := writeGeneratedConfig(*output, starterConfigForProvider(provider, options)); err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: write config: %v\n", err)
		return 2
	}

	if *ciMode {
		_, _ = fmt.Fprintf(stdout, "wrote CI starter config to %s\n", *output)
		return 0
	}

	profilePath, err := profilepkg.Path()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: resolve profile path: %v\n", err)
		return 2
	}

	_, _ = fmt.Fprintf(stdout, "stored %s credentials in %s\n", provider.Name, profilePath)
	_, _ = fmt.Fprintf(stdout, "wrote starter config to %s\n", *output)
	return 0
}

func setupAgentCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup agent", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", defaultAgentConfigPath, "Path to write the generated agent cleanr config")
	force := fs.Bool("force", false, "Overwrite an existing output file")
	nameFlag := fs.String("name", "", "Logical agent name used in reports")
	systemPromptFlag := fs.String("system-prompt", "", "System prompt to inject into the generated scenarios")
	userPromptFlag := fs.String("user-prompt", "", "Primary user prompt for the happy-path scenario")
	providerFlag := fs.String("provider", "", "Provider override: openai or anthropic")
	modelFlag := fs.String("model", "", "Provider model override")
	apiKeyFlag := fs.String("api-key", "", "Provider API key to store locally if a provider must be configured")
	apiKeyEnvFlag := fs.String("api-key-env", "", "Environment variable name used by the generated config")
	apiModeFlag := fs.String("api-mode", "", "OpenAI API mode override")
	baseURLFlag := fs.String("base-url", "", "Optional provider base URL override")
	maxTokensFlag := fs.Int("max-tokens", 0, "Optional Anthropic max_tokens override")
	profileFlag := fs.String("profile", "", "Starter profile: pr, main, or release")
	trendGatePreset := fs.String("trend-gate-preset", "", "Optional trend gate preset override: strict, moderate, or exploratory")
	withBraintrust := fs.Bool("with-braintrust", false, "Include Braintrust result publishing and remote trend comparison using standard env-based secrets")
	braintrustProject := fs.String("braintrust-project", "", "Braintrust project name for native result publishing")
	braintrustExperiment := fs.String("braintrust-experiment", defaultIntegrationFamily, "Braintrust experiment family name")
	braintrustAPIKeyEnv := fs.String("braintrust-api-key-env", defaultBraintrustKeyEnv, "Environment variable name used for the Braintrust API key")
	braintrustBaseURL := fs.String("braintrust-base-url", "", "Optional Braintrust base URL override")
	withLangfuse := fs.Bool("with-langfuse", false, "Include Langfuse result publishing using standard env-based secrets")
	langfusePublicKeyEnv := fs.String("langfuse-public-key-env", defaultLangfusePublicEnv, "Environment variable name used for the Langfuse public key")
	langfuseSecretKeyEnv := fs.String("langfuse-secret-key-env", defaultLangfuseSecretEnv, "Environment variable name used for the Langfuse secret key")
	langfuseBaseURL := fs.String("langfuse-base-url", "", "Optional Langfuse base URL override")
	langfuseExperiment := fs.String("langfuse-experiment", defaultIntegrationFamily, "Langfuse trace family name")
	withPostHog := fs.Bool("with-posthog", false, "Include PostHog result publishing using standard env-based secrets")
	posthogTokenEnv := fs.String("posthog-project-token-env", defaultPostHogTokenEnv, "Environment variable name used for the PostHog project token")
	posthogBaseURL := fs.String("posthog-base-url", "", "Optional PostHog base URL override")
	posthogExperiment := fs.String("posthog-experiment", defaultIntegrationFamily, "PostHog event family name")
	withWebhook := fs.Bool("with-webhook", false, "Include generic webhook result publishing")
	webhookEndpoint := fs.String("webhook-endpoint", "", "Webhook endpoint URL for generic result publishing")
	webhookAPIKeyEnv := fs.String("webhook-api-key-env", defaultWebhookTokenEnv, "Environment variable name used for the optional webhook bearer token")
	withAttestation := fs.Bool("with-attestation", false, "Enable signed release-gate attestations using a key from an environment variable")
	attestationKeyEnv := fs.String("attestation-key-env", defaultAttestationKeyEnv, "Environment variable name used for the attestation signing key")
	attestationKeyID := fs.String("attestation-key-id", defaultAttestationKeyID, "Stable key identifier embedded in generated attestations")
	ciMode := fs.Bool("ci", false, "Generate config non-interactively for CI and do not store credentials locally")
	browserMode := fs.Bool("browser", false, "Open the provider browser dashboard automatically during interactive setup")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := ensureWritableOutput(*output, *force); err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	providerOverride := profilepkg.Provider{
		Name:      *providerFlag,
		Model:     *modelFlag,
		APIKey:    *apiKeyFlag,
		APIKeyEnv: *apiKeyEnvFlag,
		APIMode:   *apiModeFlag,
		BaseURL:   *baseURLFlag,
		MaxTokens: *maxTokensFlag,
	}

	options, err := resolveStarterConfigOptions(starterConfigOptions{
		Profile:              *profileFlag,
		TrendGatePreset:      *trendGatePreset,
		WithBraintrust:       *withBraintrust,
		BraintrustProject:    *braintrustProject,
		BraintrustExperiment: *braintrustExperiment,
		BraintrustAPIKeyEnv:  *braintrustAPIKeyEnv,
		BraintrustBaseURL:    *braintrustBaseURL,
		WithLangfuse:         *withLangfuse,
		LangfusePublicKeyEnv: *langfusePublicKeyEnv,
		LangfuseSecretKeyEnv: *langfuseSecretKeyEnv,
		LangfuseBaseURL:      *langfuseBaseURL,
		LangfuseExperiment:   *langfuseExperiment,
		WithPostHog:          *withPostHog,
		PostHogTokenEnv:      *posthogTokenEnv,
		PostHogBaseURL:       *posthogBaseURL,
		PostHogExperiment:    *posthogExperiment,
		WithWebhook:          *withWebhook,
		WebhookEndpoint:      *webhookEndpoint,
		WebhookAPIKeyEnv:     *webhookAPIKeyEnv,
		WithAttestation:      *withAttestation,
		AttestationKeyEnv:    *attestationKeyEnv,
		AttestationKeyID:     *attestationKeyID,
	}, *ciMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	prompter := newSetupPrompter(stdin, stdout, *ciMode)
	provider, err := resolveAgentProvider(prompter, providerOverride, *ciMode, *browserMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	agentName, systemPrompt, userPrompt, err := gatherAgentInputs(prompter, *nameFlag, *systemPromptFlag, *userPromptFlag, *ciMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: %v\n", err)
		return 2
	}

	if err := writeGeneratedConfig(*output, starterAgentConfig(provider, agentName, systemPrompt, userPrompt, options)); err != nil {
		_, _ = fmt.Fprintf(stderr, "setup error: write config: %v\n", err)
		return 2
	}

	if *ciMode {
		_, _ = fmt.Fprintf(stdout, "wrote CI agent config to %s\n", *output)
		return 0
	}

	_, _ = fmt.Fprintf(stdout, "wrote agent config to %s\n", *output)
	_, _ = fmt.Fprintf(stdout, "next step: cleanr snapshot -config %s\n", *output)
	return 0
}
