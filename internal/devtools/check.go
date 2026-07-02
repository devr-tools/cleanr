package devtools

import (
	"context"
	"fmt"
)

func (r Runner) Lint(ctx context.Context) error {
	if _, err := fmt.Fprintln(r.Stdout, "running go vet"); err != nil {
		return err
	}
	return r.runCommand(ctx, nil, "go", "vet", "./...")
}

func (r Runner) Test(ctx context.Context) error {
	if _, err := fmt.Fprintln(r.Stdout, "running go test -race"); err != nil {
		return err
	}
	// -race so concurrency bugs in the engines and test suite fail the gate
	// instead of shipping; -shuffle surfaces hidden inter-test ordering
	// dependencies.
	return r.runGoTestFiltered(ctx, "-race", "-shuffle=on", "./...")
}

func (r Runner) TestReviewUI(ctx context.Context) error {
	if _, err := fmt.Fprintln(r.Stdout, "running review UI tests"); err != nil {
		return err
	}
	return r.runGoTestFiltered(ctx, "-run", "Test(DatasetReviewCommandInteractiveModeAppliesScenarioEdits|DatasetReviewTUIModelSupportsNavigationAndActions|DatasetReviewTUIViewUsesStructuredLayout)$", "./tests/cli")
}

func (r Runner) Check(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{name: "gofiles", fn: func(context.Context) error { return r.CheckGoFiles() }},
		{name: "fmt-check", fn: r.FormatCheck},
		{name: "lint", fn: r.Lint},
		{name: "test", fn: r.Test},
	}
	for _, step := range steps {
		if _, err := fmt.Fprintf(r.Stdout, "==> %s\n", step.name); err != nil {
			return err
		}
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}
	return nil
}
