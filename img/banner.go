package img

import (
	_ "embed"
	"encoding/base64"
	"strings"
)

//go:embed cleanr.txt
var cleanrBanner string

//go:embed cleanrApproved.txt
var cleanrApprovedBanner string

//go:embed cleanr.png
var cleanrLogoPNG []byte

func Banner() string {
	return trimBanner(cleanrBanner)
}

func ApprovedBanner() string {
	return trimBanner(cleanrApprovedBanner)
}

func LogoDataURL() string {
	if len(cleanrLogoPNG) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(cleanrLogoPNG)
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
