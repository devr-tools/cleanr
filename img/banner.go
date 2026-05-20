package img

import (
	_ "embed"
	"strings"
)

//go:embed cleanr.txt
var cleanrBanner string

func Banner() string {
	lines := strings.Split(cleanrBanner, "\n")
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
