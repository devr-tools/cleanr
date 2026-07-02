package toolkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

// MCPAllowExecEnv opts a config out of the MCP execution sandbox. When set to a
// truthy value the MCP surface will run configs that execute external code
// (cli targets, plugins, state adapters, probes). It defaults to off so that a
// config supplied through the MCP tools cannot achieve arbitrary command
// execution or secret exfiltration on the host.
const MCPAllowExecEnv = "CLEANR_MCP_ALLOW_EXEC"

// GuardMCPConfig rejects configs reached through the MCP surface that would
// execute attacker-influenceable external code. A config with a cli target runs
// exec.Command, and a config that declares plugins/state_adapters/probes loads
// and runs external code, all inheriting the full host environment. Unless the
// operator has explicitly opted in via MCPAllowExecEnv, such configs are
// refused with a clear error before they can run.
func GuardMCPConfig(cfg cleanr.Config) error {
	if mcpExecAllowed() {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Target.Type), "cli") {
		return fmt.Errorf("cleanr MCP refuses configs with a %q target because it executes local commands; set %s=1 to allow", "cli", MCPAllowExecEnv)
	}
	if len(cfg.Plugins) > 0 {
		return fmt.Errorf("cleanr MCP refuses configs that declare plugins because they execute external code; set %s=1 to allow", MCPAllowExecEnv)
	}
	for _, manifest := range cfg.ResolvedPlugins {
		if len(manifest.StateAdapters) > 0 || len(manifest.Probes) > 0 || len(manifest.Suites) > 0 {
			return fmt.Errorf("cleanr MCP refuses configs that declare plugins/state_adapters/probes because they execute external code; set %s=1 to allow", MCPAllowExecEnv)
		}
	}
	return nil
}

func mcpExecAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(MCPAllowExecEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// secureLocalPath confines a client-supplied path argument to the process
// working directory. Absolute paths and paths that traverse outside the working
// directory are rejected with a generic error that never echoes any file
// content, and the returned path is guaranteed to resolve within the working
// directory.
func secureLocalPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("invalid path")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("path must be relative to the working directory")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must not traverse outside the working directory")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to resolve working directory")
	}
	resolved := filepath.Join(cwd, cleaned)
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must not traverse outside the working directory")
	}
	return resolved, nil
}
