package cleanr

import (
	configpkg "cleanr/cleanr/config"
	"cleanr/cleanr/core"
)

type Config = core.Config
type TargetConfig = core.TargetConfig
type OpenAIConfig = core.OpenAIConfig
type Scenario = core.Scenario
type SuitesConfig = core.SuitesConfig
type PromptInjectionConfig = core.PromptInjectionConfig
type SecurityConfig = core.SecurityConfig
type LoadConfig = core.LoadConfig
type ChaosConfig = core.ChaosConfig
type DriftConfig = core.DriftConfig
type TokenOptimizationConfig = core.TokenOptimizationConfig
type ReportingConfig = core.ReportingConfig
type Request = core.Request
type Response = core.Response
type TokenUsage = core.TokenUsage
type Finding = core.Finding
type CaseResult = core.CaseResult
type SuiteResult = core.SuiteResult
type Report = core.Report
type Target = core.Target
type Engine = core.Engine
type RunContext = core.RunContext
type FieldError = configpkg.FieldError
type ValidationErrors = configpkg.ValidationErrors

func LoadConfigFile(path string) (Config, error) {
	return configpkg.LoadConfigFile(path)
}

func WriteConfigFile(path string, cfg Config) error {
	return configpkg.WriteConfigFile(path, cfg)
}

func ValidateConfig(cfg Config) error {
	return configpkg.ValidateConfig(cfg)
}

func ExampleConfig() Config {
	return configpkg.ExampleConfig()
}
