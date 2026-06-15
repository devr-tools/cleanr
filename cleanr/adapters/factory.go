package adapters

import (
	"context"
	"fmt"
	"net/http"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func NewTargetFromConfig(cfg core.TargetConfig, client *http.Client) core.Target {
	var target core.Target
	switch cfg.TargetType() {
	case "openai", "openai_compatible":
		target = NewOpenAI(cfg, client)
	case "cli":
		target = NewCLI(cfg)
	case "graphql":
		target = NewGraphQL(cfg, client)
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
