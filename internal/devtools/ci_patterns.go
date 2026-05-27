package devtools

import "regexp"

var (
	ciCodeChangePattern    = regexp.MustCompile(`^(cleanr/|cmd/|internal/).+\.go$`)
	ciCodeIgnorePattern    = regexp.MustCompile(`(^|/).+_test\.go$|(^|/)doc\.go$|^internal/version/version\.go$`)
	ciTestChangePattern    = regexp.MustCompile(`^(tests/|.*_test\.go$)`)
	ciCICDOnlyPattern      = regexp.MustCompile(`^(\.github/workflows/|\.github/release-please-config\.json$|\.release-please-manifest\.json$)`)
	ciDocSensitivePattern  = regexp.MustCompile(`^(\.goreleaser\.yaml$|\.github/release-please-config\.json$|\.release-please-manifest\.json$|Formula/|README\.md$|docs/|internal/devtools/)`)
	ciDocsChangedPattern   = regexp.MustCompile(`^(README\.md|CONTRIBUTING\.md|docs/)`)
	ciCriticalFilesPattern = regexp.MustCompile(`^(cmd/cleanr/|cmd/cleanr-dev/|cleanr/|internal/cli/|internal/devtools/|internal/mcpserver/|go\.mod|go\.sum|Formula/|\.goreleaser\.yaml|\.github/workflows/.*\.yml|\.github/release-please-config\.json|\.release-please-manifest\.json)`)
	ciDangerousAddPattern  = regexp.MustCompile(`^\+.*(exec\.Command(Context)?\(|http\.(Get|Post)\(|net\.Dial\(|os\.RemoveAll\(|os\.Setenv\(|syscall\.|unsafe \{|panic!\(|TODO|FIXME)`)
	ciGoModAdditionPattern = regexp.MustCompile(`^\+[^+]`)
)

type gocycloFinding struct {
	Complexity int
	Path       string
	Package    string
	Symbol     string
	Raw        string
}
