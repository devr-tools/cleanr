package devtools

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Runner struct {
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

type ReleaseOptions struct {
	Version string
	Output  string
}

type Platform struct {
	GOOS   string
	GOARCH string
}

func NewRunner(workDir string, stdout, stderr io.Writer) Runner {
	return Runner{
		WorkDir: workDir,
		Stdout:  stdout,
		Stderr:  stderr,
	}
}

func (r Runner) ListGoFiles() error {
	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if _, err := fmt.Fprintln(r.Stdout, file); err != nil {
			return err
		}
	}
	return nil
}

func (r Runner) CheckGoFiles() error {
	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no Go files found")
	}
	if err := validateGoFileLayout(files); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(r.Stdout, "go file layout: ok"); err != nil {
		return err
	}
	return nil
}

func (r Runner) Format(ctx context.Context) error {
	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	if err := validateGoFileLayout(files); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no Go files found")
	}
	if _, err := fmt.Fprintln(r.Stdout, "formatting Go files"); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, "gofmt", append([]string{"-w"}, files...)...)
}

func (r Runner) FormatCheck(ctx context.Context) error {
	files, err := discoverGoFiles(r.WorkDir)
	if err != nil {
		return err
	}
	if err := validateGoFileLayout(files); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no Go files found")
	}
	path, err := exec.LookPath("gofmt")
	if err != nil {
		return fmt.Errorf("find gofmt: %w", err)
	}
	cmd := exec.CommandContext(ctx, path, append([]string{"-l"}, files...)...)
	cmd.Dir = r.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run gofmt -l: %w", err)
	}
	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		return fmt.Errorf("unformatted Go files:\n%s", trimmed)
	}
	if _, err := fmt.Fprintln(r.Stdout, "format check: ok"); err != nil {
		return err
	}
	return nil
}

func (r Runner) Lint(ctx context.Context) error {
	if _, err := fmt.Fprintln(r.Stdout, "running go vet"); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, "go", "vet", "./...")
}

func (r Runner) Test(ctx context.Context) error {
	if _, err := fmt.Fprintln(r.Stdout, "running go test"); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, "go", "test", "./...")
}

func (r Runner) Build(ctx context.Context, output string) error {
	if output == "" {
		output = filepath.Join("dist", "cleanr")
	}
	outputPath := resolvePath(r.WorkDir, output)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create build output dir: %w", err)
	}
	if _, err := fmt.Fprintf(r.Stdout, "building %s\n", outputPath); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, "go", "build", "-trimpath", "-o", outputPath, "./cmd/cleanr")
}

func (r Runner) Check(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{name: "gofiles", fn: func(context.Context) error { return r.CheckGoFiles() }},
		{name: "fmt-check", fn: r.FormatCheck},
		{name: "lint", fn: r.Lint},
		{name: "test", fn: r.Test},
	}
	for _, step := range steps {
		if _, err := fmt.Fprintf(r.Stdout, "==> %s\n", step.name); err != nil {
			return err
		}
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}
	return nil
}

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

func (r Runner) runCommand(ctx context.Context, env map[string]string, name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("find %s: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = r.WorkDir
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func discoverGoFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".gocache", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func validateGoFileLayout(files []string) error {
	for _, file := range files {
		switch {
		case strings.HasPrefix(file, "cleanr/"):
		case strings.HasPrefix(file, "cmd/"):
		case strings.HasPrefix(file, "internal/"):
		case strings.HasPrefix(file, "tests/") && strings.HasSuffix(file, "_test.go"):
		default:
			return fmt.Errorf("unexpected Go file location: %s", file)
		}
		if strings.HasSuffix(file, "_test.go") && !strings.HasPrefix(file, "tests/") {
			return fmt.Errorf("test file must live under tests/: %s", file)
		}
		if strings.HasPrefix(file, "tests/") && !strings.HasSuffix(file, "_test.go") {
			return fmt.Errorf("non-test Go file cannot live under tests/: %s", file)
		}
	}
	return nil
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

func resolvePath(workDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}
