package snapshots

import (
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type File struct {
	Version     string             `json:"version"`
	GeneratedAt time.Time          `json:"generated_at"`
	Target      string             `json:"target"`
	Scenarios   []ScenarioSnapshot `json:"scenarios"`
}

type ScenarioSnapshot struct {
	Name       string                `json:"name"`
	System     string                `json:"system,omitempty"`
	Input      string                `json:"input,omitempty"`
	StatusCode int                   `json:"status_code,omitempty"`
	Text       string                `json:"text,omitempty"`
	Usage      core.TokenUsage       `json:"usage,omitempty"`
	Normalized core.ProviderResponse `json:"normalized,omitempty"`
}

func (f File) FindScenario(name string) (ScenarioSnapshot, bool) {
	for _, scenario := range f.Scenarios {
		if scenario.Name == name {
			return scenario, true
		}
	}
	return ScenarioSnapshot{}, false
}
