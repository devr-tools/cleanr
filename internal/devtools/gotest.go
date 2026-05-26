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

type testKey struct {
	pkg  string
	test string
}

type goTestCollector struct {
	passCount              int
	failCount              int
	testOutput             map[testKey][]string
	packageOutput          map[string][]string
	packageHasNamedFailure map[string]bool
	failureSections        []string
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

	collector := newGoTestCollector()

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
			collector.handleTestEvent(event)
		} else {
			collector.handlePackageEvent(event)
		}
	}

	waitErr := cmd.Wait()
	if len(collector.failureSections) > 0 {
		if _, err := fmt.Fprintln(r.Stdout, "failed tests:"); err != nil {
			return err
		}
		for _, section := range collector.failureSections {
			if _, err := fmt.Fprintf(r.Stdout, "\n%s", section); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(r.Stdout, "test summary: %d passed, %d failed\n", collector.passCount, collector.failCount); err != nil {
		return err
	}
	if waitErr != nil {
		return fmt.Errorf("go %s: %w", strings.Join(cmdArgs, " "), waitErr)
	}
	return nil
}

func newGoTestCollector() *goTestCollector {
	return &goTestCollector{
		testOutput:             make(map[testKey][]string),
		packageOutput:          make(map[string][]string),
		packageHasNamedFailure: make(map[string]bool),
	}
}

func (c *goTestCollector) handleTestEvent(event goTestEvent) {
	key := testKey{pkg: event.Package, test: event.Test}
	switch event.Action {
	case "output":
		c.testOutput[key] = append(c.testOutput[key], event.Output)
	case "pass":
		c.passCount++
		delete(c.testOutput, key)
	case "fail":
		c.recordNamedFailure(event.Package, event.Test, c.testOutput[key])
		delete(c.testOutput, key)
	case "skip":
		delete(c.testOutput, key)
	}
}

func (c *goTestCollector) recordNamedFailure(pkg, test string, lines []string) {
	c.failCount++
	c.packageHasNamedFailure[pkg] = true
	var section strings.Builder
	fmt.Fprintf(&section, "[%s] %s\n", pkg, test)
	for _, line := range lines {
		_, _ = io.WriteString(&section, line)
	}
	c.failureSections = append(c.failureSections, section.String())
}

func (c *goTestCollector) handlePackageEvent(event goTestEvent) {
	switch event.Action {
	case "output":
		if strings.Contains(event.Output, "[no test files]") {
			return
		}
		c.packageOutput[event.Package] = append(c.packageOutput[event.Package], event.Output)
	case "pass", "skip":
		delete(c.packageOutput, event.Package)
	case "fail":
		c.recordPackageFailure(event.Package)
		delete(c.packageOutput, event.Package)
	}
}

func (c *goTestCollector) recordPackageFailure(pkg string) {
	lines := filterPackageFailureOutput(c.packageOutput[pkg])
	if c.packageHasNamedFailure[pkg] || len(lines) == 0 {
		return
	}
	var section strings.Builder
	fmt.Fprintf(&section, "[%s]\n", pkg)
	for _, line := range lines {
		_, _ = io.WriteString(&section, line)
	}
	c.failureSections = append(c.failureSections, section.String())
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
