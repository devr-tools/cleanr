package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

type explainOutput struct {
	FailureID      string                      `json:"failure_id"`
	Suite          string                      `json:"suite"`
	Case           string                      `json:"case"`
	Failed         bool                        `json:"failed"`
	Summary        string                      `json:"summary"`
	Findings       []cleanr.Finding            `json:"findings,omitempty"`
	Scenario       *cleanr.ScenarioFingerprint `json:"scenario,omitempty"`
	Evidence       map[string]any              `json:"evidence,omitempty"`
	FixSuggestions []explainFixSuggestion      `json:"fix_suggestions,omitempty"`
}

type explainFixSuggestion struct {
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Actions    []string `json:"actions,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
}

func explainCmd(args []string, stdout, stderr io.Writer) int {
	failureID := ""
	if len(args) > 0 && !strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
		failureID = strings.TrimSpace(args[0])
		args = args[1:]
	}

	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Optional cleanr config path used to resolve the replay artifact")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	replayPath := fs.String("replay-artifact", "", "Optional replay artifact path")
	format := fs.String("format", "text", "Explain output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if failureID == "" {
		if len(remaining) == 1 {
			failureID = strings.TrimSpace(remaining[0])
		}
	}
	if failureID == "" || len(remaining) > 1 {
		_, _ = fmt.Fprintln(stderr, "explain error: provide exactly one failure id such as security/case-1")
		return 2
	}
	replayArtifactPath, err := resolveExplainReplayArtifactPath(*configPath, *profile, *replayPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "explain error: %v\n", err)
		return 2
	}

	artifact, err := cleanr.LoadReplayArtifactFile(replayArtifactPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "explain error: %v\n", err)
		return 2
	}
	entry, ok := findReplayFailure(artifact, failureID)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "explain error: failure %s not found in %s\n", failureID, replayArtifactPath)
		return 2
	}

	output := buildExplainOutput(entry)
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "text":
		writeExplainText(stdout, output)
		return 0
	case "json":
		return writeJSON(stdout, output)
	default:
		_, _ = fmt.Fprintf(stderr, "explain error: unsupported format %s\n", *format)
		return 2
	}
}

func resolveExplainReplayArtifactPath(configPath, profile, replayPath string) (string, error) {
	if strings.TrimSpace(replayPath) != "" {
		if strings.TrimSpace(configPath) != "" || strings.TrimSpace(profile) != "" {
			resolvedConfigPath, err := resolveConfigPath(configPath, profile)
			if err == nil {
				return resolveConfigRelativePath(resolvedConfigPath, replayPath), nil
			}
		}
		return replayPath, nil
	}

	resolvedConfigPath, err := resolveConfigPath(configPath, profile)
	if err != nil {
		return "", err
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Reporting.ReplayArtifactFile) == "" {
		return "", fmt.Errorf("no replay artifact configured; pass -replay-artifact or set reporting.replay_artifact_file")
	}
	return resolveConfigRelativePath(resolvedConfigPath, cfg.Reporting.ReplayArtifactFile), nil
}

func findReplayFailure(artifact cleanr.ReplayArtifact, failureID string) (cleanr.ReplayArtifactCase, bool) {
	for _, item := range artifact.Failures {
		id := replayFailureID(item)
		if failureID == id || failureID == item.Name || failureID == item.Suite+":"+item.Name {
			return item, true
		}
	}
	return cleanr.ReplayArtifactCase{}, false
}

func replayFailureID(item cleanr.ReplayArtifactCase) string {
	return item.Suite + "/" + item.Name
}

func buildExplainOutput(item cleanr.ReplayArtifactCase) explainOutput {
	out := explainOutput{
		FailureID:      replayFailureID(item),
		Suite:          item.Suite,
		Case:           item.Name,
		Failed:         item.Failed,
		Findings:       append([]cleanr.Finding(nil), item.Findings...),
		Scenario:       item.Scenario,
		Evidence:       cloneExplainEvidence(item.Evidence),
		FixSuggestions: explainFixSuggestions(item),
	}
	out.Summary = explainSummary(item)
	return out
}

func explainSummary(item cleanr.ReplayArtifactCase) string {
	if len(item.Findings) > 0 {
		return item.Findings[0].Message
	}
	if item.Failed {
		return "scenario failed without structured findings"
	}
	return "scenario produced non-fatal findings"
}

func explainFixSuggestions(item cleanr.ReplayArtifactCase) []explainFixSuggestion {
	if len(item.Findings) == 0 && len(item.Evidence) == 0 {
		return nil
	}
	var out []explainFixSuggestion
	for _, finding := range item.Findings {
		lower := strings.ToLower(strings.TrimSpace(finding.Message))
		switch {
		case strings.Contains(lower, "unsupported claim"),
			strings.Contains(lower, "claimed tool execution with no matching invocation"),
			strings.Contains(lower, "no matching invocation"):
			out = append(out, explainFixSuggestion{
				Kind:       "trace_alignment",
				Title:      "Align claimed tool or citation behavior with trace evidence",
				Actions:    []string{"Inspect scenario assertions and prompt instructions for claimed actions that never occurred", "Update the target prompt or tool wiring so claims are backed by observed invocations"},
				Confidence: "high",
			})
		case strings.Contains(lower, "semantic drift"):
			out = append(out, explainFixSuggestion{
				Kind:       "stability",
				Title:      "Reduce response instability for this scenario",
				Actions:    []string{"Inspect recent prompt or model changes in the drift build diff", "Tighten scenario instructions or add a stronger reference/rubric for the unstable output"},
				Confidence: "medium",
			})
		case strings.Contains(lower, "secret") || strings.Contains(lower, "pii"):
			out = append(out, explainFixSuggestion{
				Kind:       "policy_hardening",
				Title:      "Harden the target against sensitive-data leakage",
				Actions:    []string{"Add stricter refusal guidance for sensitive data", "Review tool outputs and prompt context for accidental secret exposure"},
				Confidence: "high",
			})
		}
	}
	if len(out) == 0 {
		out = append(out, explainFixSuggestion{
			Kind:       "investigate",
			Title:      "Inspect the recorded findings and evidence for this failure",
			Actions:    []string{"Review the structured evidence keys in the replay artifact", "Re-run the scenario with json or agent report output to capture additional context"},
			Confidence: "low",
		})
	}
	return out
}

func writeExplainText(w io.Writer, output explainOutput) {
	_, _ = fmt.Fprintf(w, "Failure ID  %s\n", output.FailureID)
	_, _ = fmt.Fprintf(w, "Suite       %s\n", output.Suite)
	_, _ = fmt.Fprintf(w, "Case        %s\n", output.Case)
	if output.Failed {
		_, _ = fmt.Fprintln(w, "Status      FAIL")
	} else {
		_, _ = fmt.Fprintln(w, "Status      WARN")
	}
	_, _ = fmt.Fprintf(w, "Summary     %s\n", output.Summary)
	if output.Scenario != nil {
		_, _ = fmt.Fprintf(w, "Scenario    %s\n", output.Scenario.Name)
	}
	if len(output.Findings) > 0 {
		_, _ = fmt.Fprintln(w, "\nFindings")
		for _, finding := range output.Findings {
			_, _ = fmt.Fprintf(w, "- %s: %s\n", strings.ToUpper(strings.TrimSpace(finding.Severity)), finding.Message)
		}
	}
	if len(output.Evidence) > 0 {
		_, _ = fmt.Fprintln(w, "\nEvidence")
		keys := sortedStringKeys(output.Evidence)
		for _, key := range keys {
			data, _ := json.Marshal(output.Evidence[key])
			_, _ = fmt.Fprintf(w, "- %s: %s\n", key, strings.TrimSpace(string(data)))
		}
	}
	if len(output.FixSuggestions) > 0 {
		_, _ = fmt.Fprintln(w, "\nFix Suggestions")
		for _, suggestion := range output.FixSuggestions {
			_, _ = fmt.Fprintf(w, "- %s [%s]\n", suggestion.Title, suggestion.Confidence)
			for _, action := range suggestion.Actions {
				_, _ = fmt.Fprintf(w, "  %s\n", action)
			}
		}
	}
}

func cloneExplainEvidence(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func sortedStringKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
