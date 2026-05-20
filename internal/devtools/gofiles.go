package devtools

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

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
		case strings.HasPrefix(file, "img/"):
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
