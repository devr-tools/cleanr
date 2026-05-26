package runtime

import (
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultIntegrationFamily = "cleanr-release-gate"

func integrationFamily(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultIntegrationFamily
	}
	return name
}

func runScopeSuffix(buildID string, generatedAt time.Time) string {
	buildID = strings.TrimSpace(buildID)
	if buildID != "" {
		return buildID
	}
	return generatedAt.UTC().Format("20060102T150405Z")
}

func normalizedBaseURL(baseURL, endpoint, fallback string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	if raw := strings.TrimSpace(endpoint); raw != "" {
		if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/")
		}
	}
	return strings.TrimRight(fallback, "/")
}

func expandRunURLWithValues(tmpl string, payload SinkPayload, extra map[string]string) string {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return ""
	}
	values := map[string]string{
		"project":    payload.Project,
		"experiment": payload.Experiment,
		"build_id":   payload.BuildID,
		"target":     payload.Target,
	}
	for key, value := range extra {
		values[key] = value
	}
	replacements := make([]string, 0, len(values)*2)
	for key, value := range values {
		replacements = append(replacements, "{{"+key+"}}", value)
	}
	return strings.NewReplacer(replacements...).Replace(tmpl)
}

func envValue(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(name))
}
