package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	"gopkg.in/yaml.v3"
)

const reviewedScenarioDatasetVersion = "v1alpha1"

type ReviewedScenarioDataset struct {
	Version           string                    `json:"version"`
	Source            string                    `json:"source,omitempty"`
	InputSource       string                    `json:"input_source,omitempty"`
	Target            string                    `json:"target,omitempty"`
	BuildID           string                    `json:"build_id,omitempty"`
	PolicyPath        string                    `json:"policy_path,omitempty"`
	PolicyVersion     string                    `json:"policy_version,omitempty"`
	GeneratedAt       time.Time                 `json:"generated_at"`
	ReviewedAt        time.Time                 `json:"reviewed_at"`
	Generator         *ScenarioDatasetGenerator `json:"generator,omitempty"`
	Warnings          []string                  `json:"warnings,omitempty"`
	Summary           DatasetReviewSummary      `json:"summary"`
	Scenarios         []ReviewedScenarioEntry   `json:"scenarios"`
	ApprovedScenarios int                       `json:"approved_scenarios,omitempty"`
	RejectedScenarios int                       `json:"rejected_scenarios,omitempty"`
	PendingScenarios  int                       `json:"pending_scenarios,omitempty"`
}

type ReviewedScenarioEntry struct {
	Entry            ScenarioDatasetEntry  `json:"entry"`
	ExistingScenario *core.Scenario        `json:"existing_scenario,omitempty"`
	Diff             DatasetReviewDiff     `json:"diff"`
	Analysis         DatasetReviewAnalysis `json:"analysis"`
	Decision         DatasetReviewDecision `json:"decision"`
}

type DatasetReviewDiff struct {
	Status              string   `json:"status,omitempty"`
	ComparedTo          string   `json:"compared_to,omitempty"`
	DuplicateOf         string   `json:"duplicate_of,omitempty"`
	SystemChanged       bool     `json:"system_changed,omitempty"`
	InputChanged        bool     `json:"input_changed,omitempty"`
	MetadataChanged     bool     `json:"metadata_changed,omitempty"`
	TagsChanged         bool     `json:"tags_changed,omitempty"`
	ExpectedChanged     bool     `json:"expected_changed,omitempty"`
	ForbiddenChanged    bool     `json:"forbidden_changed,omitempty"`
	AssertionsChanged   bool     `json:"assertions_changed,omitempty"`
	ContextChanged      bool     `json:"context_changed,omitempty"`
	MemoryReplayChanged bool     `json:"memory_replay_changed,omitempty"`
	Summary             []string `json:"summary,omitempty"`
}

type DatasetReviewAnalysis struct {
	HighestSeverity      string   `json:"highest_severity,omitempty"`
	SeverityScore        int      `json:"severity_score,omitempty"`
	NoveltyScore         int      `json:"novelty_score,omitempty"`
	DuplicatePenalty     int      `json:"duplicate_penalty,omitempty"`
	StableSuitability    string   `json:"stable_suitability,omitempty"`
	StableSuitable       bool     `json:"stable_suitable,omitempty"`
	StableReasons        []string `json:"stable_reasons,omitempty"`
	UsefulnessScore      int      `json:"usefulness_score,omitempty"`
	ExactDuplicate       bool     `json:"exact_duplicate,omitempty"`
	PromoteStableDefault bool     `json:"promote_stable_default,omitempty"`
}

type DatasetReviewDecision struct {
	Status      string            `json:"status,omitempty"`
	AddedTags   []string          `json:"added_tags,omitempty"`
	SetTags     []string          `json:"set_tags,omitempty"`
	SetMetadata map[string]string `json:"set_metadata,omitempty"`
	PolicyRules []string          `json:"policy_rules,omitempty"`
}

type DatasetReviewSummary struct {
	TotalCandidates int `json:"total_candidates"`
	NewCandidates   int `json:"new_candidates,omitempty"`
	Modified        int `json:"modified,omitempty"`
	Duplicates      int `json:"duplicates,omitempty"`
	Unchanged       int `json:"unchanged,omitempty"`
}

type DatasetReviewOptions struct {
	Approve           []string
	Reject            []string
	PromoteStable     []string
	PromoteRegression []string
	AddTags           map[string][]string
	SetTags           map[string][]string
	SetMetadata       map[string]map[string]string
	Policy            *DatasetReviewPolicy
}

func LoadReviewedScenarioDatasetFile(path string) (ReviewedScenarioDataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReviewedScenarioDataset{}, err
	}
	return LoadReviewedScenarioDatasetData(data, path)
}

func LoadReviewedScenarioDatasetData(data []byte, path string) (ReviewedScenarioDataset, error) {
	if isYAMLPath(path) {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return ReviewedScenarioDataset{}, fmt.Errorf("decode reviewed scenario dataset: %w", err)
		}
		normalized := normalizeYAMLValue(generic)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return ReviewedScenarioDataset{}, fmt.Errorf("decode reviewed scenario dataset: %w", err)
		}
		var reviewed ReviewedScenarioDataset
		if err := json.Unmarshal(raw, &reviewed); err != nil {
			return ReviewedScenarioDataset{}, fmt.Errorf("decode reviewed scenario dataset: %w", err)
		}
		return reviewed, nil
	}
	var reviewed ReviewedScenarioDataset
	if err := json.Unmarshal(data, &reviewed); err != nil {
		return ReviewedScenarioDataset{}, fmt.Errorf("decode reviewed scenario dataset: %w", err)
	}
	return reviewed, nil
}

func WriteReviewedScenarioDatasetFile(path string, reviewed ReviewedScenarioDataset) error {
	data, err := encodeReviewedScenarioDataset(reviewed, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ReviewDatasetAgainstConfig(base core.Config, dataset ScenarioDataset, opts DatasetReviewOptions) (ReviewedScenarioDataset, error) {
	if err := validateReviewOptions(dataset, opts); err != nil {
		return ReviewedScenarioDataset{}, err
	}

	index := buildReviewIndex(base)
	reviewed := newReviewedScenarioDataset(dataset)
	policyCtx := datasetReviewPolicyContext{
		Source: dataset.Source,
	}
	if dataset.Generator != nil {
		policyCtx.GeneratorProvider = dataset.Generator.Provider
		policyCtx.GeneratorModel = dataset.Generator.Model
	}
	for _, item := range dataset.Scenarios {
		entry := reviewScenario(item, opts, index, policyCtx)
		reviewed.Scenarios = append(reviewed.Scenarios, entry)
		accumulateReviewSummary(&reviewed, entry)
	}
	sortReviewedScenarios(reviewed.Scenarios)
	reviewed.Summary.TotalCandidates = len(reviewed.Scenarios)
	return reviewed, nil
}

func ApprovedDatasetFromReview(reviewed ReviewedScenarioDataset) ScenarioDataset {
	out := ScenarioDataset{
		Version:     reviewed.Version,
		Source:      reviewed.InputSource,
		Target:      reviewed.Target,
		BuildID:     reviewed.BuildID,
		GeneratedAt: reviewed.ReviewedAt.UTC(),
		Generator:   reviewed.Generator,
	}
	for _, item := range reviewed.Scenarios {
		if item.Decision.Status != "approved" {
			continue
		}
		entry := item.Entry
		entry.Scenario.Metadata = withReviewProvenance(entry.Scenario.Metadata, reviewed, item)
		out.Scenarios = append(out.Scenarios, entry)
	}
	return out
}

func MergeReviewedDatasetIntoConfig(base core.Config, reviewed ReviewedScenarioDataset) core.Config {
	return MergeDatasetIntoConfig(base, ApprovedDatasetFromReview(reviewed))
}

type datasetReviewIndex struct {
	existingByName map[string]core.Scenario
	existingByBody map[string][]core.Scenario
}

func buildReviewIndex(base core.Config) datasetReviewIndex {
	index := datasetReviewIndex{
		existingByName: make(map[string]core.Scenario, len(base.Scenarios)),
		existingByBody: make(map[string][]core.Scenario, len(base.Scenarios)),
	}
	for _, scenario := range base.Scenarios {
		index.existingByName[scenario.Name] = scenario
		key := scenarioBodyKey(scenario)
		index.existingByBody[key] = append(index.existingByBody[key], scenario)
	}
	return index
}

func newReviewedScenarioDataset(dataset ScenarioDataset) ReviewedScenarioDataset {
	return ReviewedScenarioDataset{
		Version:     reviewedScenarioDatasetVersion,
		Source:      "cleanr-review",
		InputSource: dataset.Source,
		Target:      dataset.Target,
		BuildID:     dataset.BuildID,
		GeneratedAt: dataset.GeneratedAt.UTC(),
		ReviewedAt:  time.Now().UTC(),
		Generator:   dataset.Generator,
		Warnings:    append([]string(nil), dataset.Warnings...),
		Scenarios:   make([]ReviewedScenarioEntry, 0, len(dataset.Scenarios)),
	}
}

func reviewScenario(item ScenarioDatasetEntry, opts DatasetReviewOptions, index datasetReviewIndex, policyCtx datasetReviewPolicyContext) ReviewedScenarioEntry {
	baseEntry := buildReviewedEntry(item, index.existingByName, index.existingByBody)
	candidate, decision := applyReviewOptions(item, opts, baseEntry, policyCtx)
	entry := buildReviewedEntry(candidate, index.existingByName, index.existingByBody)
	entry.Decision = decision
	return entry
}

func applyReviewOptions(item ScenarioDatasetEntry, opts DatasetReviewOptions, baseEntry ReviewedScenarioEntry, policyCtx datasetReviewPolicyContext) (ScenarioDatasetEntry, DatasetReviewDecision) {
	candidate := item
	decision := DatasetReviewDecision{Status: "pending"}
	applyPolicyRules(&candidate, &decision, baseEntry, opts.Policy, policyCtx)
	addedTags := addReviewTags(&candidate, item, opts)
	setTags := setScenarioTags(&candidate, item, opts)
	setReviewMetadata(&candidate, item, opts, &decision)
	if status := resolveReviewDecisionStatus(item.Scenario.Name, opts); status != "" {
		decision.Status = status
	}
	finalizeReviewDecision(&decision, addedTags, setTags)
	sort.Strings(candidate.Scenario.Tags)
	return candidate, decision
}

func addReviewTags(candidate *ScenarioDatasetEntry, item ScenarioDatasetEntry, opts DatasetReviewOptions) []string {
	var addedTags []string
	for _, tag := range opts.AddTags[item.Scenario.Name] {
		if containsString(candidate.Scenario.Tags, tag) {
			continue
		}
		candidate.Scenario.Tags = append(candidate.Scenario.Tags, tag)
		addedTags = append(addedTags, tag)
	}
	addedTags = addPromotionTag(candidate, addedTags, opts.PromoteStable, item.Scenario.Name, "stable")
	return addPromotionTag(candidate, addedTags, opts.PromoteRegression, item.Scenario.Name, "regression")
}

func addPromotionTag(candidate *ScenarioDatasetEntry, addedTags, selected []string, name, tag string) []string {
	if !containsString(selected, name) || containsString(candidate.Scenario.Tags, tag) {
		return addedTags
	}
	candidate.Scenario.Tags = append(candidate.Scenario.Tags, tag)
	return append(addedTags, tag)
}

func setScenarioTags(candidate *ScenarioDatasetEntry, item ScenarioDatasetEntry, opts DatasetReviewOptions) []string {
	tags := opts.SetTags[item.Scenario.Name]
	if len(tags) == 0 {
		return nil
	}
	candidate.Scenario.Tags = append([]string(nil), tags...)
	return append([]string(nil), tags...)
}

func setReviewMetadata(candidate *ScenarioDatasetEntry, item ScenarioDatasetEntry, opts DatasetReviewOptions, decision *DatasetReviewDecision) {
	metadata := opts.SetMetadata[item.Scenario.Name]
	if len(metadata) == 0 {
		return
	}
	if candidate.Scenario.Metadata == nil {
		candidate.Scenario.Metadata = map[string]string{}
	}
	for key, value := range metadata {
		candidate.Scenario.Metadata[key] = value
	}
	decision.SetMetadata = cloneStringMap(metadata)
}

func resolveReviewDecisionStatus(name string, opts DatasetReviewOptions) string {
	switch {
	case containsString(opts.Reject, name):
		return "rejected"
	case containsString(opts.Approve, name):
		return "approved"
	default:
		return ""
	}
}

func finalizeReviewDecision(decision *DatasetReviewDecision, addedTags, setTags []string) {
	if len(addedTags) > 0 {
		sort.Strings(addedTags)
		decision.AddedTags = addedTags
	}
	if len(setTags) > 0 {
		sort.Strings(setTags)
		decision.SetTags = setTags
	}
}

func accumulateReviewSummary(reviewed *ReviewedScenarioDataset, entry ReviewedScenarioEntry) {
	switch entry.Decision.Status {
	case "approved":
		reviewed.ApprovedScenarios++
	case "rejected":
		reviewed.RejectedScenarios++
	default:
		reviewed.PendingScenarios++
	}
	switch entry.Diff.Status {
	case "new":
		reviewed.Summary.NewCandidates++
	case "modified":
		reviewed.Summary.Modified++
	case "duplicate":
		reviewed.Summary.Duplicates++
	case "unchanged":
		reviewed.Summary.Unchanged++
	}
}

func sortReviewedScenarios(scenarios []ReviewedScenarioEntry) {
	sort.SliceStable(scenarios, func(i, j int) bool {
		left := scenarios[i]
		right := scenarios[j]
		if left.Analysis.UsefulnessScore != right.Analysis.UsefulnessScore {
			return left.Analysis.UsefulnessScore > right.Analysis.UsefulnessScore
		}
		if left.Analysis.SeverityScore != right.Analysis.SeverityScore {
			return left.Analysis.SeverityScore > right.Analysis.SeverityScore
		}
		return left.Entry.Scenario.Name < right.Entry.Scenario.Name
	})
}

func encodeReviewedScenarioDataset(reviewed ReviewedScenarioDataset, path string) ([]byte, error) {
	if isYAMLPath(path) {
		raw, err := json.Marshal(reviewed)
		if err != nil {
			return nil, fmt.Errorf("encode reviewed scenario dataset: %w", err)
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("encode reviewed scenario dataset: %w", err)
		}
		data, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("encode reviewed scenario dataset: %w", err)
		}
		return data, nil
	}
	data, err := json.MarshalIndent(reviewed, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode reviewed scenario dataset: %w", err)
	}
	return data, nil
}
