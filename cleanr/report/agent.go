package report

import (
	"encoding/json"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
	typespkg "github.com/devr-tools/cleanr/cleanr/core/types"
)

const (
	agentReportKind    = "cleanr.report.agent"
	agentReportFormat  = "agent"
	agentReportVersion = "v1"
)

func renderAgentReport(report core.Report) ([]byte, error) {
	encodable := typespkg.AgentReport{
		Contract: typespkg.AgentOutputContract{
			Kind:    agentReportKind,
			Format:  agentReportFormat,
			Version: agentReportVersion,
		},
		Summary: typespkg.AgentReportSummary{
			Target:              report.Name,
			Passed:              report.Passed,
			GeneratedAt:         report.GeneratedAt,
			Duration:            report.Duration,
			TotalSuites:         report.TotalSuites,
			FailedSuites:        report.FailedSuites,
			TotalCases:          report.TotalCases,
			FailedCases:         report.FailedCases,
			FindingCount:        countAgentFindings(report),
			RecommendationCount: len(report.Recommendations),
		},
		Findings:        flattenAgentFindings(report),
		FixSuggestions:  buildAgentFixSuggestions(report),
		Recommendations: report.Recommendations,
		Report:          report,
	}
	return json.MarshalIndent(encodable, "", "  ")
}

func countAgentFindings(report core.Report) int {
	return len(flattenAgentFindings(report))
}

func flattenAgentFindings(report core.Report) []typespkg.AgentFinding {
	findings := make([]typespkg.AgentFinding, 0)
	for _, suite := range report.Suites {
		for _, finding := range suite.Findings {
			findings = append(findings, typespkg.AgentFinding{
				ID:       suite.Name,
				Scope:    "suite",
				Suite:    suite.Name,
				Severity: finding.Severity,
				Message:  finding.Message,
			})
		}
		for _, c := range suite.Cases {
			for _, finding := range c.Findings {
				findings = append(findings, typespkg.AgentFinding{
					ID:       suite.Name + "/" + c.Name,
					Scope:    "case",
					Suite:    suite.Name,
					Case:     c.Name,
					Severity: finding.Severity,
					Message:  finding.Message,
					Details:  c.Details,
				})
			}
		}
	}
	if report.TrendGate != nil {
		for _, finding := range report.TrendGate.Findings {
			findings = append(findings, typespkg.AgentFinding{
				ID:       "trend_gate",
				Scope:    "trend_gate",
				Severity: finding.Severity,
				Message:  finding.Message,
			})
		}
	}
	return findings
}

func buildAgentFixSuggestions(report core.Report) []typespkg.AgentFixSuggestion {
	suggestions := make([]typespkg.AgentFixSuggestion, 0)
	for _, suite := range report.Suites {
		for _, finding := range suite.Findings {
			suggestions = append(suggestions, inferAgentFixSuggestion("suite", suite.Name, "", finding.Message)...)
		}
		for _, c := range suite.Cases {
			for _, finding := range c.Findings {
				suggestions = append(suggestions, inferAgentFixSuggestion("case", suite.Name, c.Name, finding.Message)...)
				if detailsMessage := agentCaseDetailsMessage(c.Details); detailsMessage != "" {
					suggestions = append(suggestions, inferAgentFixSuggestion("case", suite.Name, c.Name, detailsMessage)...)
				}
			}
		}
	}
	if report.TrendGate != nil {
		for _, finding := range report.TrendGate.Findings {
			suggestions = append(suggestions, inferAgentFixSuggestion("trend_gate", "", "", finding.Message)...)
		}
	}
	return compactAgentFixSuggestions(suggestions)
}

func agentCaseDetailsMessage(details map[string]any) string {
	if len(details) == 0 {
		return ""
	}
	if value, ok := details["first_unsupported_claim"].(string); ok {
		return value
	}
	return ""
}

func inferAgentFixSuggestion(scope, suite, caseName, message string) []typespkg.AgentFixSuggestion {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "unsupported claim"),
		strings.Contains(lower, "claimed tool execution with no matching invocation"),
		strings.Contains(lower, "no matching invocation"):
		return []typespkg.AgentFixSuggestion{{
			ID:         fixSuggestionID(scope, suite, caseName, "trace_alignment"),
			Scope:      scope,
			Suite:      suite,
			Case:       caseName,
			Kind:       "trace_alignment",
			Title:      "Align claimed tool or citation behavior with trace evidence",
			Actions:    []string{"Inspect the prompt, rubric, and assertions for claims that are not backed by observed tool traces", "Update tool wiring or response instructions so claimed actions are supported by recorded evidence"},
			Confidence: "high",
		}}
	case strings.Contains(lower, "semantic drift"):
		return []typespkg.AgentFixSuggestion{{
			ID:         fixSuggestionID(scope, suite, caseName, "stability"),
			Scope:      scope,
			Suite:      suite,
			Case:       caseName,
			Kind:       "stability",
			Title:      "Reduce response instability for this scenario",
			Actions:    []string{"Inspect the build diff and recent prompt or model changes tied to this scenario", "Tighten the scenario instructions or add stronger references to reduce output variance"},
			Confidence: "medium",
		}}
	case strings.Contains(lower, "secret"), strings.Contains(lower, "pii"):
		return []typespkg.AgentFixSuggestion{{
			ID:         fixSuggestionID(scope, suite, caseName, "policy_hardening"),
			Scope:      scope,
			Suite:      suite,
			Case:       caseName,
			Kind:       "policy_hardening",
			Title:      "Harden the target against sensitive-data leakage",
			Actions:    []string{"Strengthen refusal and redaction guidance for sensitive data", "Inspect tool outputs and injected context for accidental secret exposure paths"},
			Confidence: "high",
		}}
	default:
		return nil
	}
}

func compactAgentFixSuggestions(items []typespkg.AgentFixSuggestion) []typespkg.AgentFixSuggestion {
	if len(items) == 0 {
		return nil
	}
	out := make([]typespkg.AgentFixSuggestion, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func fixSuggestionID(scope, suite, caseName, kind string) string {
	parts := []string{scope}
	if suite != "" {
		parts = append(parts, suite)
	}
	if caseName != "" {
		parts = append(parts, caseName)
	}
	parts = append(parts, kind)
	return strings.Join(parts, "/")
}
