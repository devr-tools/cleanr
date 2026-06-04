package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const datasetReviewPolicyVersion = "v1alpha1"

type DatasetReviewPolicy struct {
	Version string                    `json:"version"`
	Rules   []DatasetReviewPolicyRule `json:"rules,omitempty"`
}

type DatasetReviewPolicyRule struct {
	Name                string            `json:"name,omitempty"`
	Action              string            `json:"action"`
	Statuses            []string          `json:"statuses,omitempty"`
	Sources             []string          `json:"sources,omitempty"`
	GeneratorProviders  []string          `json:"generator_providers,omitempty"`
	GeneratorModels     []string          `json:"generator_models,omitempty"`
	ScenarioTags        []string          `json:"scenario_tags,omitempty"`
	MinSeverity         string            `json:"min_severity,omitempty"`
	StableSuitability   string            `json:"stable_suitability,omitempty"`
	RequireAssertions   bool              `json:"require_assertions,omitempty"`
	RequireExpectedText bool              `json:"require_expected_text,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

func LoadDatasetReviewPolicyFile(path string) (DatasetReviewPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DatasetReviewPolicy{}, err
	}
	return LoadDatasetReviewPolicyData(data, path)
}

func LoadDatasetReviewPolicyData(data []byte, path string) (DatasetReviewPolicy, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return DatasetReviewPolicy{}, fmt.Errorf("decode dataset review policy: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return DatasetReviewPolicy{}, fmt.Errorf("decode dataset review policy: %w", err)
		}
		var policy DatasetReviewPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return DatasetReviewPolicy{}, fmt.Errorf("decode dataset review policy: %w", err)
		}
		return normalizeDatasetReviewPolicy(policy)
	}
	var policy DatasetReviewPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return DatasetReviewPolicy{}, fmt.Errorf("decode dataset review policy: %w", err)
	}
	return normalizeDatasetReviewPolicy(policy)
}

func WriteDatasetReviewPolicyFile(path string, policy DatasetReviewPolicy) error {
	normalized, err := normalizeDatasetReviewPolicy(policy)
	if err != nil {
		return err
	}
	data, err := encodeDatasetReviewPolicy(normalized, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func normalizeDatasetReviewPolicy(policy DatasetReviewPolicy) (DatasetReviewPolicy, error) {
	if strings.TrimSpace(policy.Version) == "" {
		policy.Version = datasetReviewPolicyVersion
	}
	if policy.Version != datasetReviewPolicyVersion {
		return DatasetReviewPolicy{}, fmt.Errorf("dataset review policy version %q is not supported", policy.Version)
	}
	for i := range policy.Rules {
		rule := &policy.Rules[i]
		rule.Action = strings.ToLower(strings.TrimSpace(rule.Action))
		rule.MinSeverity = strings.ToLower(strings.TrimSpace(rule.MinSeverity))
		rule.StableSuitability = strings.ToLower(strings.TrimSpace(rule.StableSuitability))
		normalizeLowerStrings(rule.Statuses)
		normalizeLowerStrings(rule.Sources)
		normalizeLowerStrings(rule.GeneratorProviders)
		normalizeLowerStrings(rule.GeneratorModels)
		normalizeTrimmedStrings(rule.Tags)
		normalizeTrimmedStrings(rule.ScenarioTags)
		if err := validateDatasetReviewPolicyRule(*rule); err != nil {
			return DatasetReviewPolicy{}, fmt.Errorf("dataset review policy rule %d: %w", i, err)
		}
	}
	return policy, nil
}

func validateDatasetReviewPolicyRule(rule DatasetReviewPolicyRule) error {
	switch rule.Action {
	case "approve", "reject", "promote-stable", "promote-regression", "add-tags", "set-metadata":
	default:
		return fmt.Errorf("unsupported action %q", rule.Action)
	}
	for _, status := range rule.Statuses {
		switch status {
		case "new", "modified", "duplicate", "unchanged":
		default:
			return fmt.Errorf("unsupported status %q", status)
		}
	}
	if rule.MinSeverity != "" && severityScore(rule.MinSeverity) == 0 && rule.MinSeverity != "info" {
		return fmt.Errorf("unsupported min_severity %q", rule.MinSeverity)
	}
	if rule.StableSuitability != "" {
		switch rule.StableSuitability {
		case "low", "medium", "high":
		default:
			return fmt.Errorf("unsupported stable_suitability %q", rule.StableSuitability)
		}
	}
	if (rule.Action == "promote-stable" || rule.Action == "promote-regression") && len(rule.Tags) > 0 {
		return fmt.Errorf("tags are only supported with action add-tags")
	}
	if rule.Action == "add-tags" && len(rule.Tags) == 0 {
		return fmt.Errorf("action add-tags requires at least one tag")
	}
	if rule.Action == "set-metadata" && len(rule.Metadata) == 0 {
		return fmt.Errorf("action set-metadata requires at least one metadata entry")
	}
	return nil
}

func encodeDatasetReviewPolicy(policy DatasetReviewPolicy, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("encode dataset review policy: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode dataset review policy: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode dataset review policy: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode dataset review policy: %w", err)
	}
	return data, nil
}

func applyPolicyRules(candidate *ScenarioDatasetEntry, decision *DatasetReviewDecision, baseEntry ReviewedScenarioEntry, policy *DatasetReviewPolicy, policyCtx datasetReviewPolicyContext) {
	if policy == nil {
		return
	}
	for i, rule := range policy.Rules {
		if !reviewPolicyRuleMatches(rule, baseEntry, policyCtx) {
			continue
		}
		decision.PolicyRules = append(decision.PolicyRules, datasetReviewPolicyRuleLabel(rule, i))
		switch rule.Action {
		case "approve":
			decision.Status = "approved"
		case "reject":
			decision.Status = "rejected"
		case "promote-stable":
			addPolicyTag(candidate, decision, "stable")
		case "promote-regression":
			addPolicyTag(candidate, decision, "regression")
		case "add-tags":
			for _, tag := range rule.Tags {
				addPolicyTag(candidate, decision, tag)
			}
		case "set-metadata":
			if candidate.Scenario.Metadata == nil {
				candidate.Scenario.Metadata = map[string]string{}
			}
			if decision.SetMetadata == nil {
				decision.SetMetadata = map[string]string{}
			}
			for key, value := range rule.Metadata {
				candidate.Scenario.Metadata[key] = value
				decision.SetMetadata[key] = value
			}
		}
	}
}

func datasetReviewPolicyRuleLabel(rule DatasetReviewPolicyRule, index int) string {
	if name := strings.TrimSpace(rule.Name); name != "" {
		return name
	}
	return fmt.Sprintf("rule-%d:%s", index+1, rule.Action)
}

type datasetReviewPolicyContext struct {
	Source            string
	GeneratorProvider string
	GeneratorModel    string
}

func reviewPolicyRuleMatches(rule DatasetReviewPolicyRule, entry ReviewedScenarioEntry, policyCtx datasetReviewPolicyContext) bool {
	if len(rule.Statuses) > 0 && !containsString(rule.Statuses, entry.Diff.Status) {
		return false
	}
	if len(rule.Sources) > 0 && !containsString(rule.Sources, strings.ToLower(strings.TrimSpace(policyCtx.Source))) {
		return false
	}
	if len(rule.GeneratorProviders) > 0 && !containsString(rule.GeneratorProviders, strings.ToLower(strings.TrimSpace(policyCtx.GeneratorProvider))) {
		return false
	}
	if len(rule.GeneratorModels) > 0 && !containsString(rule.GeneratorModels, strings.ToLower(strings.TrimSpace(policyCtx.GeneratorModel))) {
		return false
	}
	if len(rule.ScenarioTags) > 0 && !containsAllTags(entry.Entry.Scenario.Tags, rule.ScenarioTags) {
		return false
	}
	if rule.MinSeverity != "" && entry.Analysis.SeverityScore < severityScore(rule.MinSeverity) {
		return false
	}
	if rule.StableSuitability != "" && stableSuitabilityRank(entry.Analysis.StableSuitability) < stableSuitabilityRank(rule.StableSuitability) {
		return false
	}
	if rule.RequireAssertions && len(entry.Entry.Scenario.Assertions) == 0 {
		return false
	}
	if rule.RequireExpectedText && len(entry.Entry.Scenario.ExpectedContains) == 0 && len(entry.Entry.Scenario.ForbiddenContains) == 0 {
		return false
	}
	return true
}

func addPolicyTag(candidate *ScenarioDatasetEntry, decision *DatasetReviewDecision, tag string) {
	tag = strings.TrimSpace(tag)
	if tag == "" || containsString(candidate.Scenario.Tags, tag) {
		return
	}
	candidate.Scenario.Tags = append(candidate.Scenario.Tags, tag)
	decision.AddedTags = append(decision.AddedTags, tag)
}

func stableSuitabilityRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func normalizeLowerStrings(values []string) {
	for i, value := range values {
		values[i] = strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeTrimmedStrings(values []string) {
	for i, value := range values {
		values[i] = strings.TrimSpace(value)
	}
}

func containsAllTags(have []string, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, required := range want {
		if !containsString(have, required) {
			return false
		}
	}
	return true
}
