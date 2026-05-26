package devtools

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxExtractedBinarySize = 256 * 1024 * 1024

func installBinaryFromArchivePath(archivePath, toolPath, binaryName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open %s archive %s: %w", binaryName, archivePath, err)
	}
	defer f.Close()

	if err := os.MkdirAll(filepath.Dir(toolPath), 0o755); err != nil {
		return fmt.Errorf("create %s bin dir: %w", binaryName, err)
	}
	tmpPath := toolPath + ".tmp"
	if err := extractBinaryFromTarGz(f, tmpPath, binaryName); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod %s binary: %w", binaryName, err)
	}
	if err := os.Rename(tmpPath, toolPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install %s binary: %w", binaryName, err)
	}
	return nil
}

func extractBinaryFromTarGz(src io.Reader, outputPath, binaryName string) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("open %s archive: %w", binaryName, err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read %s archive: %w", binaryName, err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		if header.Size < 0 {
			return fmt.Errorf("%s archive entry has negative size", binaryName)
		}
		if header.Size > maxExtractedBinarySize {
			return fmt.Errorf("%s binary exceeds max size of %d bytes", binaryName, maxExtractedBinarySize)
		}
		f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("create %s binary: %w", binaryName, err)
		}
		if _, err := io.CopyN(f, tr, header.Size); err != nil {
			_ = f.Close()
			return fmt.Errorf("write %s binary: %w", binaryName, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s binary: %w", binaryName, err)
		}
		return nil
	}
	return fmt.Errorf("%s binary not found in archive", binaryName)
}
