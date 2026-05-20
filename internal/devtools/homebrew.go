package devtools

import (
	"bufio"
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

	checksumsPath := strings.TrimSpace(opts.Checksums)
	if checksumsPath == "" {
		checksumsPath = filepath.Join("dist", "releases", tag, "SHA256SUMS")
	}
	checksums, err := loadReleaseChecksums(resolvePath(r.WorkDir, checksumsPath))
	if err != nil {
		return err
	}

	formula, err := renderHomebrewFormula(tag, repo, checksums)
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

func loadReleaseChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open checksums: %w", err)
	}
	defer file.Close()

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line: %q", line)
		}
		checksums[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	return checksums, nil
}

func renderHomebrewFormula(tag, repo string, checksums map[string]string) (string, error) {
	darwinAMD64Name := fmt.Sprintf("cleanr_%s_darwin_amd64.tar.gz", tag)
	darwinARM64Name := fmt.Sprintf("cleanr_%s_darwin_arm64.tar.gz", tag)
	linuxAMD64Name := fmt.Sprintf("cleanr_%s_linux_amd64.tar.gz", tag)
	linuxARM64Name := fmt.Sprintf("cleanr_%s_linux_arm64.tar.gz", tag)

	darwinAMD64, ok := checksums[darwinAMD64Name]
	if !ok {
		return "", fmt.Errorf("missing checksum for %s", darwinAMD64Name)
	}
	darwinARM64, ok := checksums[darwinARM64Name]
	if !ok {
		return "", fmt.Errorf("missing checksum for %s", darwinARM64Name)
	}
	linuxAMD64, ok := checksums[linuxAMD64Name]
	if !ok {
		return "", fmt.Errorf("missing checksum for %s", linuxAMD64Name)
	}
	linuxARM64, ok := checksums[linuxARM64Name]
	if !ok {
		return "", fmt.Errorf("missing checksum for %s", linuxARM64Name)
	}

	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, tag)
	version := strings.TrimPrefix(tag, "v")

	return fmt.Sprintf(`class Cleanr < Formula
  desc "AI testing SDK and CLI for CI validation"
  homepage "https://github.com/%s"
  version "%s"

  on_macos do
    if Hardware::CPU.arm?
      url "%s/%s"
      sha256 "%s"
    else
      url "%s/%s"
      sha256 "%s"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "%s/%s"
      sha256 "%s"
    else
      url "%s/%s"
      sha256 "%s"
    end
  end

  def install
    bin.install "cleanr"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/cleanr version")
  end
end
`, repo, version,
		baseURL, darwinARM64Name, darwinARM64,
		baseURL, darwinAMD64Name, darwinAMD64,
		baseURL, linuxARM64Name, linuxARM64,
		baseURL, linuxAMD64Name, linuxAMD64,
	), nil
}
