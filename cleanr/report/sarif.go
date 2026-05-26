package report

import (
	"encoding/json"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID               string                 `json:"id"`
	ShortDescription map[string]string      `json:"shortDescription,omitempty"`
	Properties       map[string]interface{} `json:"properties,omitempty"`
}

type sarifResult struct {
	RuleID     string                 `json:"ruleId"`
	Level      string                 `json:"level"`
	Message    map[string]string      `json:"message"`
	Locations  []sarifLocationWrapper `json:"locations,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type sarifLocationWrapper struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

func renderSARIF(report core.Report) ([]byte, error) {
	rules := make(map[string]sarifRule)
	results := make([]sarifResult, 0)

	addFinding := func(suite string, caseName string, finding core.Finding, details map[string]any) {
		ruleID := "cleanr." + sanitizeRuleID(suite)
		if caseName != "" {
			ruleID += "." + sanitizeRuleID(caseName)
		}
		ruleID += "." + sanitizeRuleID(finding.Severity)
		if _, ok := rules[ruleID]; !ok {
			rules[ruleID] = sarifRule{
				ID:               ruleID,
				ShortDescription: map[string]string{"text": finding.Message},
				Properties: map[string]interface{}{
					"suite": suite,
				},
			}
		}
		properties := map[string]interface{}{
			"suite":    suite,
			"severity": finding.Severity,
		}
		if caseName != "" {
			properties["case"] = caseName
		}
		results = append(results, sarifResult{
			RuleID:     ruleID,
			Level:      sarifLevel(finding.Severity),
			Message:    map[string]string{"text": finding.Message},
			Locations:  sarifLocations(details),
			Properties: properties,
		})
	}

	for _, suite := range report.Suites {
		for _, finding := range suite.Findings {
			addFinding(suite.Name, "", finding, suite.Meta)
		}
		for _, c := range suite.Cases {
			for _, finding := range c.Findings {
				addFinding(suite.Name, c.Name, finding, c.Details)
			}
		}
	}
	if report.TrendGate != nil {
		for _, finding := range report.TrendGate.Findings {
			addFinding("trend-gate", "", finding, nil)
		}
	}

	ruleList := make([]sarifRule, 0, len(rules))
	for _, rule := range rules {
		ruleList = append(ruleList, rule)
	}

	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:  "cleanr",
					Rules: ruleList,
				},
			},
			Results: results,
		}},
	}
	return json.MarshalIndent(log, "", "  ")
}

func sarifLevel(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	default:
		return "note"
	}
}

func sanitizeRuleID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" {
		return "unknown"
	}
	return value
}

func sarifLocations(details map[string]any) []sarifLocationWrapper {
	paths := make([]string, 0)
	for _, key := range []string{"changed_files", "approved_changes", "unexpected_changes", "observed_file_changes", "missing_expected_mutations", "undeclared_approved_changes"} {
		paths = append(paths, detailStringSlice(details, key)...)
	}
	if len(paths) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(paths))
	out := make([]sarifLocationWrapper, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok || strings.TrimSpace(path) == "" {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, sarifLocationWrapper{
			PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifactLocation{URI: path},
			},
		})
	}
	return out
}

func detailStringSlice(details map[string]any, key string) []string {
	if len(details) == 0 {
		return nil
	}
	value, ok := details[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
