package devtools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (r Runner) HomebrewFormula(opts HomebrewFormulaOptions) error {
	tag := strings.TrimSpace(opts.Version)
	if tag == "" {
		return fmt.Errorf("version is required")
	}

	repo := strings.TrimSpace(opts.Repository)
	if repo == "" {
		return fmt.Errorf("repository is required")
	}

	sourceSHA256 := strings.TrimSpace(opts.SourceSHA256)
	if sourceSHA256 == "" {
		return fmt.Errorf("source SHA256 is required")
	}

	formula, err := renderHomebrewFormula(tag, repo, sourceSHA256, strings.TrimSpace(opts.License))
	if err != nil {
		return err
	}

	output := strings.TrimSpace(opts.Output)
	if output == "" {
		output = filepath.Join("dist", "releases", tag, "cleanr.rb")
	}
	outputPath := resolvePath(r.WorkDir, output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create formula output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(formula), 0o644); err != nil {
		return fmt.Errorf("write formula: %w", err)
	}
	_, err = fmt.Fprintf(r.Stdout, "wrote Homebrew formula %s\n", outputPath)
	return err
}

func renderHomebrewFormula(tag, repo, sourceSHA256, license string) (string, error) {
	version := strings.TrimPrefix(tag, "v")
	licenseLine := ""
	if license != "" {
		licenseLine = fmt.Sprintf("  license %q\n", license)
	}

	return fmt.Sprintf(`class Cleanr < Formula
  desc "AI testing SDK and CLI for CI validation"
  homepage "https://github.com/%s"
  url "https://github.com/%s/archive/refs/tags/%s.tar.gz"
  sha256 "%s"
  version "%s"
%s  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/devr-tools/cleanr/internal/cli.version=#{version}"
    system "go", "build", *std_go_args(output: bin/"cleanr", ldflags: ldflags), "./cmd/cleanr"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/cleanr version")
  end
end
`, repo, repo, tag, sourceSHA256, version, licenseLine), nil
}
