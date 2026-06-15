package core

import "time"

func BuildScenarioRequest(scenario Scenario, timeout time.Duration) Request {
	return Request{
		Scenario: scenario,
		System:   scenario.SystemValue(),
		Prompt:   scenario.InputValue(),
		Messages: scenario.TurnsValue(),
		Timeout:  timeout,
	}
}
