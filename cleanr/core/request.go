package core

import "time"

func BuildScenarioRequest(scenario Scenario, timeout time.Duration) Request {
	return Request{
		Scenario:     scenario,
		System:       scenario.SystemValue(),
		Prompt:       scenario.InputValue(),
		Messages:     scenario.TurnsValue(),
		Images:       scenario.ImagesValue(),
		Audio:        scenario.AudioValue(),
		PDFs:         scenario.PDFsValue(),
		JudgeOutputs: scenario.JudgeOutputsValue(),
		Timeout:      timeout,
	}
}
