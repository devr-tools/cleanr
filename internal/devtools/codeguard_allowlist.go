package devtools

import (
	"os"
	"path/filepath"
	"strings"
)

const codeGuardGodFilesAllowlistPath = ".codeguard-godfiles-allowlist"

func loadCodeGuardGodFilesAllowlist(workDir string) (map[string]struct{}, error) {
	path := filepath.Join(workDir, codeGuardGodFilesAllowlistPath)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}

	allowlist := make(map[string]struct{})
	for _, line := range strings.Split(string(body), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		allowlist[filepath.ToSlash(entry)] = struct{}{}
	}
	return allowlist, nil
}
