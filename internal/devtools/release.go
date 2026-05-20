package devtools

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (r Runner) Release(ctx context.Context, opts ReleaseOptions) error {
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "dev"
	}
	outputRoot := strings.TrimSpace(opts.Output)
	if outputRoot == "" {
		outputRoot = filepath.Join("dist", "releases")
	}

	releaseDir := resolvePath(r.WorkDir, filepath.Join(outputRoot, version))
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		return fmt.Errorf("create release dir: %w", err)
	}

	platforms := []Platform{
		{GOOS: "darwin", GOARCH: "amd64"},
		{GOOS: "darwin", GOARCH: "arm64"},
		{GOOS: "linux", GOARCH: "amd64"},
		{GOOS: "linux", GOARCH: "arm64"},
	}

	checksums := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		artifactName := fmt.Sprintf("cleanr_%s_%s_%s", version, platform.GOOS, platform.GOARCH)
		stageDir := filepath.Join(releaseDir, artifactName)
		if err := os.MkdirAll(stageDir, 0o755); err != nil {
			return fmt.Errorf("create artifact dir: %w", err)
		}
		binaryPath := filepath.Join(stageDir, "cleanr")
		if _, err := fmt.Fprintf(r.Stdout, "releasing %s/%s\n", platform.GOOS, platform.GOARCH); err != nil {
			return err
		}
		env := map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        platform.GOOS,
			"GOARCH":      platform.GOARCH,
		}
		ldflags := fmt.Sprintf("-s -w -X cleanr/internal/cli.version=%s", version)
		if err := r.runCommand(ctx, env, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", binaryPath, "./cmd/cleanr"); err != nil {
			return fmt.Errorf("build %s/%s: %w", platform.GOOS, platform.GOARCH, err)
		}
		archivePath := filepath.Join(releaseDir, artifactName+".tar.gz")
		sum, err := archiveTarGz(archivePath, stageDir, "cleanr")
		if err != nil {
			return fmt.Errorf("archive %s: %w", artifactName, err)
		}
		checksums = append(checksums, sum+"  "+filepath.Base(archivePath))
		if err := os.RemoveAll(stageDir); err != nil {
			return fmt.Errorf("cleanup %s: %w", stageDir, err)
		}
	}

	checksumFile := filepath.Join(releaseDir, "SHA256SUMS")
	if err := os.WriteFile(checksumFile, []byte(strings.Join(checksums, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write checksums: %w", err)
	}
	_, err := fmt.Fprintf(r.Stdout, "release artifacts written to %s\n", releaseDir)
	return err
}

func archiveTarGz(outputPath, sourceDir, binaryName string) (string, error) {
	out, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	hash := sha256.New()
	multiWriter := io.MultiWriter(out, hash)
	gz := gzip.NewWriter(multiWriter)
	tw := tar.NewWriter(gz)

	binaryPath := filepath.Join(sourceDir, binaryName)
	info, err := os.Stat(binaryPath)
	if err != nil {
		return "", err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return "", err
	}
	header.Name = binaryName
	if err := tw.WriteHeader(header); err != nil {
		return "", err
	}
	file, err := os.Open(binaryPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(tw, file); err != nil {
		return "", err
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
