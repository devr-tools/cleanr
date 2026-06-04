package img

import (
	_ "embed"
	"strings"
)

//go:embed cleanr.txt
var cleanrBanner string

//go:embed cleanrApproved.txt
var cleanrApprovedBanner string

func Banner() string {
	return trimBanner(cleanrBanner)
}

func ApprovedBanner() string {
	return trimBanner(cleanrApprovedBanner)
}

func trimBanner(raw string) string {
	lines := strings.Split(raw, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}
