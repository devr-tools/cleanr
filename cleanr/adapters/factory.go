package adapters

import (
	"context"
	"fmt"
	"net/http"

	"github.com/devr-tools/cleanr/cleanr/core"
)

func NewTargetFromConfig(cfg core.TargetConfig, client *http.Client) core.Target {
	switch cfg.TargetType() {
	case "openai":
		return NewOpenAI(cfg, client)
	case "openai_compatible":
		return NewOpenAI(cfg, client)
	case "anthropic":
		return NewAnthropic(cfg, client)
	case "mcp":
		return NewMCP(cfg, client)
	case "http":
		return NewHTTP(cfg, client)
	default:
		return invalidTarget{err: fmt.Errorf("unsupported target type %q", cfg.Type)}
	}
}

type invalidTarget struct {
	err error
}

func (t invalidTarget) Invoke(context.Context, core.Request) core.Response {
	return core.Response{Err: t.err}
}
