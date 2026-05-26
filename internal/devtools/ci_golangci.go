package devtools

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func (r Runner) runCIGolangCILint(ctx context.Context, baseRef, version string) error {
	golangciLintPath, err := r.ensureGolangCILint(ctx, version)
	if err != nil {
		return err
	}

	changedFiles, err := r.gitChangedFiles(ctx, baseRef)
	if err != nil {
		return err
	}
	targets := filterCIGocycloTargets(changedFiles)
	if len(targets) == 0 {
		_, err := fmt.Fprintln(r.Stdout, "No changed non-test Go files for golangci-lint.")
		return err
	}

	mergeBase, err := r.runOutputCommand(ctx, nil, "git", "merge-base", baseRef, "HEAD")
	if err != nil {
		return err
	}
	baseline := strings.TrimSpace(mergeBase)
	if baseline == "" {
		return fmt.Errorf("empty merge-base for %s", baseRef)
	}

	if _, err := fmt.Fprintf(r.Stdout, "running golangci-lint against baseline %s\n", baseline); err != nil {
		return err
	}

	return r.runCommand(
		ctx,
		nil,
		golangciLintPath,
		"run",
		"--config",
		".golangci.yml",
		"--new-from-rev",
		baseline,
		"--whole-files",
		"./...",
	)
}

func (r Runner) ensureGolangCILint(ctx context.Context, version string) (string, error) {
	gopath, err := r.runOutputCommand(ctx, nil, "go", "env", "GOPATH")
	if err != nil {
		return "", err
	}
	toolPath := filepath.Join(strings.TrimSpace(gopath), "bin", "golangci-lint")
	if info, err := os.Stat(toolPath); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
		if _, err := fmt.Fprintf(r.Stdout, "using existing golangci-lint at %s\n", toolPath); err != nil {
			return "", err
		}
		return toolPath, nil
	}

	if _, err := fmt.Fprintf(r.Stdout, "installing golangci-lint %s\n", version); err != nil {
		return "", err
	}
	if err := r.runCommand(ctx, nil, "go", "install", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@"+version); err == nil {
		return toolPath, nil
	} else if !shouldFallbackToPrebuiltGolangCILint(err) {
		return "", err
	} else {
		if _, printErr := fmt.Fprintln(r.Stdout, "falling back to prebuilt golangci-lint binary"); printErr != nil {
			return "", printErr
		}
		if err := r.downloadGolangCILint(ctx, toolPath, version); err != nil {
			return "", err
		}
	}
	return toolPath, nil
}

func shouldFallbackToPrebuiltGolangCILint(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "invalid go version") ||
		strings.Contains(message, "unknown block type: ignore") ||
		strings.Contains(message, "unknown directive: ignore")
}

func (r Runner) downloadGolangCILint(ctx context.Context, toolPath, version string) error {
	versionTag := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if versionTag == "" {
		return fmt.Errorf("empty golangci-lint version")
	}
	archiveURL := fmt.Sprintf(
		"https://github.com/golangci/golangci-lint/releases/download/v%s/golangci-lint-%s-%s-%s.tar.gz",
		versionTag,
		versionTag,
		runtime.GOOS,
		runtime.GOARCH,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return fmt.Errorf("build golangci-lint request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download golangci-lint %s: %w", version, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download golangci-lint %s: unexpected status %s: %s", version, resp.Status, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(toolPath), 0o755); err != nil {
		return fmt.Errorf("create golangci-lint bin dir: %w", err)
	}
	tmpPath := toolPath + ".tmp"
	if err := extractGolangCILintBinary(resp.Body, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod golangci-lint binary: %w", err)
	}
	if err := os.Rename(tmpPath, toolPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install golangci-lint binary: %w", err)
	}
	return nil
}

func extractGolangCILintBinary(src io.Reader, outputPath string) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("open golangci-lint archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read golangci-lint archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "golangci-lint" {
			continue
		}
		f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("create golangci-lint binary: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return fmt.Errorf("write golangci-lint binary: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close golangci-lint binary: %w", err)
		}
		return nil
	}
	return fmt.Errorf("golangci-lint binary not found in archive")
}
