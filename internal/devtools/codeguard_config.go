package devtools

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultCodeGuardMaxFunctionLines    = 80
	defaultCodeGuardMaxFunctionParams   = 5
	defaultCodeGuardMaxNestingDepth     = 4
	defaultCodeGuardMaxFunctionReturns  = 6
	defaultCodeGuardMaxBoolParams       = 1
	defaultCodeGuardMaxDemeterChain     = 4
	defaultCodeGuardDryMinFunctionLines = 6
)

type codeGuardThresholds struct {
	MaxFunctionLines    int
	MaxFunctionParams   int
	MaxNestingDepth     int
	MaxFunctionReturns  int
	MaxBoolParams       int
	MaxDemeterChain     int
	DryMinFunctionLines int
}

func loadCodeGuardThresholds() codeGuardThresholds {
	return codeGuardThresholds{
		MaxFunctionLines:    resolveCodeGuardInt("MAX_FUNCTION_LINES", defaultCodeGuardMaxFunctionLines),
		MaxFunctionParams:   resolveCodeGuardInt("MAX_FUNCTION_PARAMS", defaultCodeGuardMaxFunctionParams),
		MaxNestingDepth:     resolveCodeGuardInt("MAX_NESTING_DEPTH", defaultCodeGuardMaxNestingDepth),
		MaxFunctionReturns:  resolveCodeGuardInt("MAX_FUNCTION_RETURNS", defaultCodeGuardMaxFunctionReturns),
		MaxBoolParams:       resolveCodeGuardInt("MAX_BOOL_PARAMS", defaultCodeGuardMaxBoolParams),
		MaxDemeterChain:     resolveCodeGuardInt("MAX_DEMETER_CHAIN", defaultCodeGuardMaxDemeterChain),
		DryMinFunctionLines: resolveCodeGuardInt("DRY_MIN_FUNCTION_LINES", defaultCodeGuardDryMinFunctionLines),
	}
}

func resolveCodeGuardInt(envName string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func codeGuardSectionBlocking(envName string, defaultValue bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(envName)))
	if raw == "" {
		return defaultValue
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}
