package cli

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

func generateAuthoringCmd(args []string, stdout, stderr io.Writer) int {
	prompt := strings.TrimSpace(args[0])
	if prompt == "" {
		_, _ = fmt.Fprintln(stderr, "generate error: natural-language prompt is required")
		return 2
	}

	fs := flag.NewFlagSet("generate author", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Optional base cleanr config to extend")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	output := fs.String("output", "cleanr.generated.yaml", "Path to write the authored cleanr config")
	name := fs.String("name", "", "Optional scenario name override")
	systemPrompt := fs.String("system", "", "Optional system prompt to attach to the authored scenario")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	cfg, resolvedConfigPath, err := loadAuthoringBaseConfig(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate error: %v\n", err)
		return 2
	}

	scenario := authoredScenario(prompt, *name, *systemPrompt, cfg)
	cfg.Scenarios = appendOrReplaceScenario(cfg.Scenarios, scenario)

	outputPath := strings.TrimSpace(*output)
	if resolvedConfigPath != "" {
		outputPath = resolveConfigRelativePath(resolvedConfigPath, outputPath)
	}
	if err := cleanr.WriteConfigFile(outputPath, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "generate error: %v\n", err)
		return 2
	}

	_, _ = fmt.Fprintf(stdout, "wrote authored scenario %s to %s\n", scenario.Name, outputPath)
	return 0
}

func loadAuthoringBaseConfig(configPath, profile string) (cleanr.Config, string, error) {
	resolvedConfigPath, err := resolveConfigPath(configPath, profile)
	if err != nil {
		if strings.TrimSpace(configPath) == "" && strings.TrimSpace(profile) == "" {
			cfg := cleanr.ExampleConfig()
			cfg.Scenarios = nil
			return cfg, "", nil
		}
		return cleanr.Config{}, "", err
	}

	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		return cleanr.Config{}, "", err
	}
	return cfg, resolvedConfigPath, nil
}

func authoredScenario(prompt, explicitName, explicitSystem string, cfg cleanr.Config) cleanr.Scenario {
	normalizedPrompt := normalizeAuthoringPrompt(prompt)
	systemPrompt := strings.TrimSpace(explicitSystem)
	if systemPrompt == "" && len(cfg.Scenarios) > 0 {
		systemPrompt = cfg.Scenarios[0].System
	}

	scenario := cleanr.Scenario{
		Name:   authoredScenarioName(normalizedPrompt, explicitName),
		System: systemPrompt,
		Input:  normalizedPrompt,
		Tags:   []string{"generated", "nl-authoring"},
		Metadata: map[string]string{
			"authoring.mode":   "natural_language",
			"authoring.prompt": strings.TrimSpace(prompt),
		},
	}
	if strings.HasPrefix(strings.ToLower(normalizedPrompt), "refuse ") || strings.Contains(strings.ToLower(normalizedPrompt), "should refuse") {
		scenario.Tags = append(scenario.Tags, "security")
	}
	return scenario
}

func appendOrReplaceScenario(scenarios []cleanr.Scenario, scenario cleanr.Scenario) []cleanr.Scenario {
	for i := range scenarios {
		if scenarios[i].Name == scenario.Name {
			scenarios[i] = scenario
			return scenarios
		}
	}
	return append(scenarios, scenario)
}

func normalizeAuthoringPrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	prefixes := []string{
		"test that ",
		"verify that ",
		"check that ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func authoredScenarioName(prompt, explicitName string) string {
	if strings.TrimSpace(explicitName) != "" {
		return strings.TrimSpace(explicitName)
	}
	base := strings.ToLower(strings.TrimSpace(prompt))
	base = nonSlugChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "authored-scenario"
	}
	if len(base) > 48 {
		base = strings.Trim(base[:48], "-")
	}
	return base
}
