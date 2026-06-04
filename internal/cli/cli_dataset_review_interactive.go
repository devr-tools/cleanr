package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devr-tools/cleanr/cleanr"
)

var interactiveTerminalAvailable = terminalUIAvailable
var interactiveIsTerminalFile = isTerminalFile

func runInteractiveDatasetReview(stdin io.Reader, stdout io.Writer, reviewed cleanr.ReviewedScenarioDataset) (cleanr.ReviewedScenarioDataset, error) {
	if session := newTTYDatasetReviewSession(stdin, stdout); session != nil {
		return session.run(reviewed)
	}
	return runLineInteractiveDatasetReview(stdin, stdout, reviewed)
}

func runLineInteractiveDatasetReview(stdin io.Reader, stdout io.Writer, reviewed cleanr.ReviewedScenarioDataset) (cleanr.ReviewedScenarioDataset, error) {
	reader := bufio.NewReader(stdin)
	_, _ = fmt.Fprintln(stdout, "interactive dataset review")
	_, _ = fmt.Fprintln(stdout, "commands: approve|a, reject|r, pending|p, stable, regression, tag <tag>, tags <a,b>, metadata <key=value>, show, help, skip, quit")
	for i := range reviewed.Scenarios {
		if err := reviewInteractiveScenario(reader, stdout, &reviewed.Scenarios[i], i+1, len(reviewed.Scenarios), reviewed.PolicyPath); err != nil {
			return cleanr.ReviewedScenarioDataset{}, err
		}
	}
	return cleanr.FinalizeReviewedScenarioDataset(reviewed), nil
}

type ttyDatasetReviewSession struct {
	in  *os.File
	out *os.File
}

func newTTYDatasetReviewSession(stdin io.Reader, stdout io.Writer) *ttyDatasetReviewSession {
	in, inOK := stdin.(*os.File)
	out, outOK := stdout.(*os.File)
	if !inOK || !outOK || !interactiveTerminalAvailable() || !interactiveIsTerminalFile(in) || !interactiveIsTerminalFile(out) {
		return nil
	}
	return &ttyDatasetReviewSession{in: in, out: out}
}

func (s *ttyDatasetReviewSession) run(reviewed cleanr.ReviewedScenarioDataset) (cleanr.ReviewedScenarioDataset, error) {
	model := newDatasetReviewTUIModel(reviewed)
	program := tea.NewProgram(
		model,
		tea.WithInput(s.in),
		tea.WithOutput(s.out),
		tea.WithAltScreen(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return cleanr.ReviewedScenarioDataset{}, err
	}
	result, ok := finalModel.(datasetReviewTUIModel)
	if !ok {
		return cleanr.ReviewedScenarioDataset{}, fmt.Errorf("unexpected review UI model type %T", finalModel)
	}
	if result.abort {
		return cleanr.ReviewedScenarioDataset{}, fmt.Errorf("interactive review aborted by user")
	}
	return cleanr.FinalizeReviewedScenarioDataset(result.reviewed), nil
}

func reviewInteractiveScenario(reader *bufio.Reader, stdout io.Writer, entry *cleanr.ReviewedScenarioEntry, index, total int, policyPath string) error {
	for {
		writeInteractiveScenarioSummary(stdout, *entry, index, total, policyPath)
		_, _ = fmt.Fprint(stdout, "review> ")
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		command := strings.TrimSpace(line)
		if command == "" {
			command = "skip"
		}
		done, quit, cmdErr := applyInteractiveReviewCommand(entry, command, stdout)
		if cmdErr != nil {
			_, _ = fmt.Fprintf(stdout, "invalid command: %v\n", cmdErr)
		}
		if quit {
			return fmt.Errorf("interactive review aborted by user")
		}
		if done || err == io.EOF {
			return nil
		}
	}
}

func writeInteractiveScenarioSummary(stdout io.Writer, entry cleanr.ReviewedScenarioEntry, index, total int, policyPath string) {
	_, _ = fmt.Fprintf(stdout, "\n[%d/%d] %s\n", index, total, entry.Entry.Scenario.Name)
	_, _ = fmt.Fprintf(stdout, "  decision=%s diff=%s score=%d severity=%s stable=%s\n",
		nonEmpty(entry.Decision.Status, "pending"),
		entry.Diff.Status,
		entry.Analysis.UsefulnessScore,
		nonEmpty(entry.Analysis.HighestSeverity, "none"),
		nonEmpty(entry.Analysis.StableSuitability, "low"),
	)
	if len(entry.Diff.Summary) > 0 {
		_, _ = fmt.Fprintf(stdout, "  changes=%s\n", strings.Join(entry.Diff.Summary, "; "))
	}
	if len(entry.Entry.Scenario.Tags) > 0 {
		_, _ = fmt.Fprintf(stdout, "  tags=%s\n", strings.Join(entry.Entry.Scenario.Tags, ","))
	}
	if len(entry.Entry.Scenario.Metadata) > 0 {
		_, _ = fmt.Fprintf(stdout, "  metadata=%s\n", formatMetadata(entry.Entry.Scenario.Metadata))
	}
	if len(entry.Decision.PolicyRules) > 0 {
		_, _ = fmt.Fprintf(stdout, "  policy_rules=%s\n", strings.Join(entry.Decision.PolicyRules, ","))
	}
	if strings.TrimSpace(policyPath) != "" {
		_, _ = fmt.Fprintf(stdout, "  policy=%s\n", policyPath)
	}
}

func applyInteractiveReviewCommand(entry *cleanr.ReviewedScenarioEntry, command string, stdout io.Writer) (done bool, quit bool, err error) {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return true, false, nil
	}
	switch strings.ToLower(fields[0]) {
	case "approve", "a":
		entry.Decision.Status = "approved"
		return true, false, nil
	case "reject", "r":
		entry.Decision.Status = "rejected"
		return true, false, nil
	case "pending", "p":
		entry.Decision.Status = "pending"
		return true, false, nil
	case "stable":
		addInteractiveTag(entry, "stable")
		entry.Decision.Status = "approved"
		return true, false, nil
	case "regression":
		addInteractiveTag(entry, "regression")
		entry.Decision.Status = "approved"
		return true, false, nil
	case "tag":
		if len(fields) < 2 {
			return false, false, fmt.Errorf("tag requires a value")
		}
		addInteractiveTag(entry, strings.Join(fields[1:], " "))
		return false, false, nil
	case "tags":
		raw := strings.TrimSpace(strings.TrimPrefix(command, fields[0]))
		if raw == "" {
			return false, false, fmt.Errorf("tags requires a comma-separated list")
		}
		setInteractiveTags(entry, splitInteractiveList(raw))
		return false, false, nil
	case "metadata":
		raw := strings.TrimSpace(strings.TrimPrefix(command, fields[0]))
		key, value, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return false, false, fmt.Errorf("metadata requires key=value")
		}
		setInteractiveMetadata(entry, strings.TrimSpace(key), strings.TrimSpace(value))
		return false, false, nil
	case "show":
		writeInteractiveScenarioSummary(stdout, *entry, 0, 0, "")
		return false, false, nil
	case "help":
		_, _ = fmt.Fprintln(stdout, "commands: approve|a, reject|r, pending|p, stable, regression, tag <tag>, tags <a,b>, metadata <key=value>, show, help, skip, quit")
		return false, false, nil
	case "skip":
		return true, false, nil
	case "quit", "q":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("unknown command %q", fields[0])
	}
}

func addInteractiveTag(entry *cleanr.ReviewedScenarioEntry, tag string) {
	tag = strings.TrimSpace(tag)
	if tag == "" || containsString(entry.Entry.Scenario.Tags, tag) {
		return
	}
	entry.Entry.Scenario.Tags = append(entry.Entry.Scenario.Tags, tag)
	sort.Strings(entry.Entry.Scenario.Tags)
	if !containsString(entry.Decision.AddedTags, tag) {
		entry.Decision.AddedTags = append(entry.Decision.AddedTags, tag)
		sort.Strings(entry.Decision.AddedTags)
	}
}

func setInteractiveTags(entry *cleanr.ReviewedScenarioEntry, tags []string) {
	entry.Entry.Scenario.Tags = append([]string(nil), tags...)
	entry.Decision.SetTags = append([]string(nil), tags...)
	sort.Strings(entry.Entry.Scenario.Tags)
	sort.Strings(entry.Decision.SetTags)
}

func setInteractiveMetadata(entry *cleanr.ReviewedScenarioEntry, key, value string) {
	if entry.Entry.Scenario.Metadata == nil {
		entry.Entry.Scenario.Metadata = map[string]string{}
	}
	if entry.Decision.SetMetadata == nil {
		entry.Decision.SetMetadata = map[string]string{}
	}
	entry.Entry.Scenario.Metadata[key] = value
	entry.Decision.SetMetadata[key] = value
}

func splitInteractiveList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	sort.Strings(out)
	return out
}

func formatMetadata(metadata map[string]string) string {
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, metadata[key]))
	}
	return strings.Join(parts, ",")
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
