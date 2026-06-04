package devtools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const sccReleaseBaseURLEnv = "CLEANR_SCC_RELEASE_BASE_URL"
const sccArchivePathEnv = "CLEANR_SCC_ARCHIVE_PATH"

type sccReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type sccRelease struct {
	Assets []sccReleaseAsset `json:"assets"`
}

type sccLanguageReport struct {
	Files []sccFileReport `json:"Files"`
}

type sccFileReport struct {
	Code     int    `json:"Code"`
	Lines    int    `json:"Lines"`
	Location string `json:"Location"`
}

func (r Runner) runCISCC(ctx context.Context, baseRef, version string, maxCodeLines int) error {
	sccPath, err := r.ensureSCC(ctx, version)
	if err != nil {
		return err
	}

	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}
	targets := filterCICodeGuardTargets(changedFiles)
	if len(targets) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No changed non-test Go files for scc.")
		return err
	}

	currentStats, err := r.runSCCReport(ctx, sccPath, r.WorkDir, targets)
	if err != nil {
		return err
	}
	baseStats, err := r.loadBaseSCCStats(ctx, baseRef, sccPath, targets)
	if err != nil {
		return err
	}

	regressions := diffSCCRegressions(currentStats, baseStats, maxCodeLines)
	if len(regressions) > 0 {
		return fmt.Errorf(
			"scc found new or worsened files above the code-line limit (%d):\n%s",
			maxCodeLines,
			strings.Join(regressions, "\n"),
		)
	}

	if _, err := fmt.Fprintf(
		r.Stdout,
		"scc: no new file-size regressions above %d code lines (%d changed files checked)\n",
		maxCodeLines,
		len(currentStats),
	); err != nil {
		return err
	}
	return nil
}

func (r Runner) ensureSCC(ctx context.Context, version string) (string, error) {
	gopath, err := r.runOutputCommand(ctx, nil, "go", "env", "GOPATH")
	if err != nil {
		return "", err
	}
	toolPath := filepath.Join(strings.TrimSpace(gopath), "bin", "scc")
	if info, err := os.Stat(toolPath); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
		if _, err := fmt.Fprintf(r.Stdout, "using existing scc at %s\n", toolPath); err != nil {
			return "", err
		}
		return toolPath, nil
	}

	if _, err := fmt.Fprintf(r.Stdout, "installing scc %s\n", version); err != nil {
		return "", err
	}
	if _, err := r.runOutputCommand(ctx, nil, "go", "install", "github.com/boyter/scc/v3@"+version); err == nil {
		return toolPath, nil
	} else if !shouldFallbackToPrebuiltGoTool(err) {
		return "", err
	} else {
		if _, printErr := fmt.Fprintln(r.Stdout, "falling back to prebuilt scc binary"); printErr != nil {
			return "", printErr
		}
		if err := r.downloadSCC(ctx, toolPath, version); err != nil {
			return "", err
		}
	}
	return toolPath, nil
}

func (r Runner) downloadSCC(ctx context.Context, toolPath, version string) error {
	if archivePath := strings.TrimSpace(os.Getenv(sccArchivePathEnv)); archivePath != "" {
		return installBinaryFromArchivePath(archivePath, toolPath, "scc")
	}

	versionTag := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if versionTag == "" {
		return fmt.Errorf("empty scc version")
	}
	baseURL := strings.TrimSpace(os.Getenv(sccReleaseBaseURLEnv))
	if baseURL == "" {
		baseURL = "https://api.github.com/repos/boyter/scc/releases/tags"
	}

	releaseURL := fmt.Sprintf("%s/v%s", strings.TrimRight(baseURL, "/"), versionTag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return fmt.Errorf("build scc release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch scc release %s: %w", version, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("fetch scc release %s: unexpected status %s: %s", version, resp.Status, strings.TrimSpace(string(body)))
	}
	var release sccRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("decode scc release %s: %w", version, err)
	}
	archiveURL, err := findSCCReleaseAssetURL(release.Assets)
	if err != nil {
		return err
	}

	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return fmt.Errorf("build scc download request: %w", err)
	}
	downloadResp, err := http.DefaultClient.Do(downloadReq)
	if err != nil {
		return fmt.Errorf("download scc %s: %w", version, err)
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(downloadResp.Body, 4096))
		return fmt.Errorf("download scc %s: unexpected status %s: %s", version, downloadResp.Status, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(toolPath), 0o755); err != nil {
		return fmt.Errorf("create scc bin dir: %w", err)
	}
	tmpPath := toolPath + ".tmp"
	if err := extractBinaryFromTarGz(downloadResp.Body, tmpPath, "scc"); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod scc binary: %w", err)
	}
	if err := os.Rename(tmpPath, toolPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install scc binary: %w", err)
	}
	return nil
}

func findSCCReleaseAssetURL(assets []sccReleaseAsset) (string, error) {
	goosTerms := []string{strings.ToLower(runtime.GOOS)}
	switch runtime.GOOS {
	case "darwin":
		goosTerms = append(goosTerms, "macos", "osx")
	}

	goarchTerms := []string{strings.ToLower(runtime.GOARCH)}
	switch runtime.GOARCH {
	case "amd64":
		goarchTerms = append(goarchTerms, "x86_64")
	case "386":
		goarchTerms = append(goarchTerms, "i386", "x86")
	case "arm64":
		goarchTerms = append(goarchTerms, "aarch64")
	}

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		if !containsAny(name, goosTerms) || !containsAny(name, goarchTerms) {
			continue
		}
		if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		return asset.BrowserDownloadURL, nil
	}

	return "", fmt.Errorf("no scc release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

func containsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func (r Runner) runSCCReport(ctx context.Context, sccPath, workDir string, targets []string) (map[string]sccFileReport, error) {
	baseRunner := NewRunner(workDir, r.Stdout, r.Stderr)
	out, err := baseRunner.runOutputCommand(ctx, nil, sccPath, append([]string{"--by-file", "--format", "json"}, targets...)...)
	if err != nil {
		return nil, err
	}
	return parseSCCReport(out)
}

func (r Runner) loadBaseSCCStats(ctx context.Context, baseRef, sccPath string, targets []string) (map[string]sccFileReport, error) {
	baseDir, err := os.MkdirTemp("", "cleanr-ci-scc-base-*")
	if err != nil {
		return nil, fmt.Errorf("create scc base temp dir: %w", err)
	}
	defer os.RemoveAll(baseDir)

	baseTargets := make([]string, 0, len(targets))
	for _, target := range targets {
		existsOut, err := r.runOutputCommand(ctx, nil, "git", "ls-tree", "-r", "--name-only", baseRef, "--", target)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(existsOut) == "" {
			continue
		}

		content, err := r.runOutputCommand(ctx, nil, "git", "show", baseRef+":"+target)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(baseDir, filepath.FromSlash(target))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create base dir for %s: %w", target, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write base file %s: %w", target, err)
		}
		baseTargets = append(baseTargets, target)
	}

	if len(baseTargets) == 0 {
		return map[string]sccFileReport{}, nil
	}

	return r.runSCCReport(ctx, sccPath, baseDir, baseTargets)
}

func parseSCCReport(raw string) (map[string]sccFileReport, error) {
	var reports []sccLanguageReport
	if err := json.Unmarshal([]byte(raw), &reports); err != nil {
		return nil, fmt.Errorf("parse scc json: %w", err)
	}

	files := make(map[string]sccFileReport)
	for _, report := range reports {
		for _, file := range report.Files {
			path := filepath.ToSlash(strings.TrimSpace(file.Location))
			if path == "" {
				continue
			}
			file.Location = path
			files[path] = file
		}
	}
	return files, nil
}

func diffSCCRegressions(current, base map[string]sccFileReport, maxCodeLines int) []string {
	regressions := make([]string, 0, len(current))
	for path, finding := range current {
		if finding.Code <= maxCodeLines {
			continue
		}

		baseline, ok := base[path]
		if ok && baseline.Code > maxCodeLines && finding.Code <= baseline.Code {
			continue
		}

		if ok {
			regressions = append(
				regressions,
				fmt.Sprintf("%s code=%d lines=%d baseline_code=%d baseline_lines=%d", path, finding.Code, finding.Lines, baseline.Code, baseline.Lines),
			)
			continue
		}

		regressions = append(regressions, fmt.Sprintf("%s code=%d lines=%d", path, finding.Code, finding.Lines))
	}
	sort.Strings(regressions)
	return regressions
}
