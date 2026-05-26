package devtools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestShouldFallbackToPrebuiltGolangCILint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "invalid go version",
			err:  errors.New("go install: invalid go version '1.25.0': must match format 1.23"),
			want: true,
		},
		{
			name: "unknown ignore block type",
			err:  errors.New("go.mod:8: unknown block type: ignore"),
			want: true,
		},
		{
			name: "unknown ignore directive",
			err:  errors.New("go.mod:5: unknown directive: ignore"),
			want: true,
		},
		{
			name: "generic install failure",
			err:  errors.New("go install: exit status 1"),
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldFallbackToPrebuiltGolangCILint(tc.err); got != tc.want {
				t.Fatalf("shouldFallbackToPrebuiltGolangCILint(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestExtractGolangCILintBinary(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "golangci-lint")
	archive := buildTestTarGz(t, map[string]string{
		"golangci-lint-2.12.2-darwin-arm64/LICENSE":       "license text",
		"golangci-lint-2.12.2-darwin-arm64/golangci-lint": "#!/bin/sh\necho ok\n",
	})

	if err := extractGolangCILintBinary(bytes.NewReader(archive), outputPath); err != nil {
		t.Fatalf("extractGolangCILintBinary: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if got, want := string(data), "#!/bin/sh\necho ok\n"; got != want {
		t.Fatalf("extracted binary contents = %q, want %q", got, want)
	}
}

func TestExtractGolangCILintBinaryMissingBinary(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "golangci-lint")
	archive := buildTestTarGz(t, map[string]string{
		"golangci-lint-2.12.2-darwin-arm64/README.md": "readme",
	})

	err := extractGolangCILintBinary(bytes.NewReader(archive), outputPath)
	if err == nil {
		t.Fatal("expected missing binary error")
	}
}

func buildTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, contents := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(contents)),
		}); err != nil {
			t.Fatalf("write tar header for %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(contents)); err != nil {
			t.Fatalf("write tar contents for %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}
