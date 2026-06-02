package devtools

import "sort"

type codeGuardStatus string

const (
	codeGuardStatusPass codeGuardStatus = "PASS"
	codeGuardStatusFail codeGuardStatus = "FAIL"
	codeGuardStatusWarn codeGuardStatus = "WARN"
	codeGuardStatusSkip codeGuardStatus = "SKIP"
)

type codeGuardViolation struct {
	Path    string
	Message string
}

type codeGuardSectionResult struct {
	Name       string
	Status     codeGuardStatus
	Violations []codeGuardViolation
	Note       string
}

func (r codeGuardSectionResult) fileCount() int {
	if len(r.Violations) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(r.Violations))
	for _, violation := range r.Violations {
		if violation.Path == "" {
			continue
		}
		seen[violation.Path] = struct{}{}
	}
	return len(seen)
}

func (r codeGuardSectionResult) blockingFailed() bool {
	return r.Status == codeGuardStatusFail
}

func sortCodeGuardViolations(violations []codeGuardViolation) {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Path == violations[j].Path {
			return violations[i].Message < violations[j].Message
		}
		return violations[i].Path < violations[j].Path
	})
}
