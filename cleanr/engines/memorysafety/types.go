package memorysafety

import (
	"fmt"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type memorySource struct {
	Name    string
	Trust   string
	Content string
	Canary  string
	Reasons []string
	UserID  string
}

type memoryReplayStep struct {
	SessionID string
	Scenario  core.Scenario
}

type memoryHazardWrite struct {
	Canary    string
	Key       string
	Reasons   []string
	SessionID string
}

type crossSessionReadMatch struct {
	Action        string
	Key           string
	Reasons       []string
	FromSessionID string
	ToSessionID   string
}

func (m crossSessionReadMatch) String() string {
	return fmt.Sprintf("%s:%s %s->%s %s", m.Action, m.Key, m.FromSessionID, m.ToSessionID, strings.Join(m.Reasons, ","))
}

type crossSessionCanaryReplay struct {
	Canary        string
	Reasons       []string
	FromSessionID string
	ToSessionID   string
}
