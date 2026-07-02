package adapters

import (
	"context"
	"fmt"
	"net/http"

	"github.com/devr-tools/cleanr/cleanr/core"
)

// SupportedTargetTypes is the single source of truth for the target/provider
// types the adapter factory can construct. Config validation and its hint
// strings derive their allowed set from this slice so the two never drift.
// The order is human-readable and is reused verbatim in validation hints.
var SupportedTargetTypes = []string{
	"cli",
	"graphql",
	"grpc",
	"openai",
	"openai_compatible",
	"azure_openai",
	"gemini",
	"bedrock",
	"vertex",
	"mistral",
	"anthropic",
	"mcp",
	"http",
}

func NewTargetFromConfig(cfg core.TargetConfig, client *http.Client) core.Target {
	var target core.Target
	switch cfg.TargetType() {
	case "openai", "openai_compatible", "azure_openai", "gemini", "bedrock", "vertex", "mistral":
		target = NewOpenAI(cfg, client)
	case "cli":
		target = NewCLI(cfg)
	case "graphql":
		target = NewGraphQL(cfg, client)
	case "grpc":
		target = NewGRPC(cfg)
	case "anthropic":
		target = NewAnthropic(cfg, client)
	case "mcp":
		target = NewMCP(cfg, client)
	case "http":
		target = NewHTTP(cfg, client)
	default:
		target = invalidTarget{err: fmt.Errorf("unsupported target type %q", cfg.Type)}
	}
	return target
}

type invalidTarget struct {
	err error
}

func (t invalidTarget) Invoke(context.Context, core.Request) core.Response {
	return core.Response{Err: t.err}
}
