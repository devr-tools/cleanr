package tools

import "fmt"

func errUnknownTool(name string) error {
	return fmt.Errorf("unknown tool: %s", name)
}
