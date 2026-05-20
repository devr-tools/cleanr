package devtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type goTestEvent struct {
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Action  string `json:"Action"`
	Output  string `json:"Output"`
}

func (r Runner) runGoTestFiltered(ctx context.Context, args ...string) error {
	path, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("find go: %w", err)
	}

	cmdArgs := append([]string{"test", "-json"}, args...)
	cmd := exec.CommandContext(ctx, path, cmdArgs...)
	cmd.Dir = r.WorkDir
	cmd.Stderr = r.Stderr
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("go test stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(cmdArgs, " "), err)
	}

	type testKey struct {
		pkg  string
		test string
	}

	var passCount int
	var failCount int
	testOutput := make(map[testKey][]string)
	packageOutput := make(map[string][]string)
	packageHasNamedFailure := make(map[string]bool)
	var failureSections []string

	dec := json.NewDecoder(stdout)
	for {
		var event goTestEvent
		if err := dec.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = cmd.Wait()
			return fmt.Errorf("decode go test json: %w", err)
		}
		if event.Test != "" {
			key := testKey{pkg: event.Package, test: event.Test}
			switch event.Action {
			case "output":
				testOutput[key] = append(testOutput[key], event.Output)
			case "pass":
				passCount++
				delete(testOutput, key)
			case "fail":
				failCount++
				packageHasNamedFailure[event.Package] = true
				var section strings.Builder
				fmt.Fprintf(&section, "[%s] %s\n", event.Package, event.Test)
				for _, line := range testOutput[key] {
					_, _ = io.WriteString(&section, line)
				}
				failureSections = append(failureSections, section.String())
				delete(testOutput, key)
			case "skip":
				delete(testOutput, key)
			}
			continue
		}

		switch event.Action {
		case "output":
			if strings.Contains(event.Output, "[no test files]") {
				continue
			}
			packageOutput[event.Package] = append(packageOutput[event.Package], event.Output)
		case "pass", "skip":
			delete(packageOutput, event.Package)
		case "fail":
			lines := filterPackageFailureOutput(packageOutput[event.Package])
			if !packageHasNamedFailure[event.Package] && len(lines) > 0 {
				var section strings.Builder
				fmt.Fprintf(&section, "[%s]\n", event.Package)
				for _, line := range lines {
					_, _ = io.WriteString(&section, line)
				}
				failureSections = append(failureSections, section.String())
			}
			delete(packageOutput, event.Package)
		}
	}

	waitErr := cmd.Wait()
	if len(failureSections) > 0 {
		if _, err := fmt.Fprintln(r.Stdout, "failed tests:"); err != nil {
			return err
		}
		for _, section := range failureSections {
			if _, err := fmt.Fprintf(r.Stdout, "\n%s", section); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(r.Stdout, "test summary: %d passed, %d failed\n", passCount, failCount); err != nil {
		return err
	}
	if waitErr != nil {
		return fmt.Errorf("go %s: %w", strings.Join(cmdArgs, " "), waitErr)
	}
	return nil
}

func filterPackageFailureOutput(lines []string) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case trimmed == "PASS":
			continue
		case trimmed == "FAIL":
			continue
		case strings.HasPrefix(line, "ok  \t"):
			continue
		case strings.HasPrefix(trimmed, "?"):
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}
