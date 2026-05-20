package devtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

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

func resolvePath(workDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}
