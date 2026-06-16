package cli

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
)

type watchOptions struct {
	run       runOptions
	interval  time.Duration
	maxRuns   int
	watchPath repeatedStringFlag
}

func watchCmd(args []string, stdout, stderr io.Writer) int {
	opts, err := parseWatchOptions(args, stderr)
	if err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(opts.run.configPath, opts.run.profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "watch error: %v\n", err)
		return 2
	}
	if opts.interval <= 0 {
		_, _ = fmt.Fprintln(stderr, "watch error: -interval must be greater than 0")
		return 2
	}

	roots := resolveWatchRoots(resolvedConfigPath, opts.watchPath)
	_, _ = fmt.Fprintf(stdout, "watching %s every %s\n", strings.Join(roots, ", "), opts.interval)

	runCount := 0
	lastExitCode := 0
	ignorePaths := make(map[string]struct{})
	var snapshot map[string]watchFileState

	for {
		runCount++
		_, _ = fmt.Fprintf(stdout, "watch run #%d at %s\n", runCount, time.Now().Format(time.RFC3339))
		lastExitCode = executeRunCommand(opts.run, stdout, stderr)
		if opts.maxRuns > 0 && runCount >= opts.maxRuns {
			return lastExitCode
		}

		cfg, cfgErr := cleanr.LoadConfigFile(resolvedConfigPath)
		if cfgErr == nil {
			ignorePaths = buildWatchIgnorePaths(resolvedConfigPath, opts.run, cfg)
		}
		snapshot, err = collectWatchSnapshot(roots, ignorePaths)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "watch error: %v\n", err)
			return 2
		}

		for {
			time.Sleep(opts.interval)
			nextSnapshot, snapErr := collectWatchSnapshot(roots, ignorePaths)
			if snapErr != nil {
				_, _ = fmt.Fprintf(stderr, "watch error: %v\n", snapErr)
				return 2
			}
			changes := diffWatchSnapshot(snapshot, nextSnapshot)
			if len(changes) == 0 {
				continue
			}
			sort.Strings(changes)
			_, _ = fmt.Fprintf(stdout, "detected changes in %s\n", strings.Join(changes, ", "))
			break
		}
	}
}

func parseWatchOptions(args []string, stderr io.Writer) (watchOptions, error) {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := watchOptions{}
	fs.StringVar(&opts.run.configPath, "config", "", "Path to cleanr config")
	fs.StringVar(&opts.run.profile, "profile", "", "Optional staged config profile: pr, main, or release")
	fs.StringVar(&opts.run.format, "format", "", "Report format: text, json, junit, sarif, or agent")
	fs.StringVar(&opts.run.output, "output", "", "Optional output file")
	fs.StringVar(&opts.run.trendFile, "trend-file", "", "Optional trend history file")
	fs.StringVar(&opts.run.replayArtifactPath, "replay-artifact", "", "Optional replay artifact file")
	fs.StringVar(&opts.run.buildID, "build-id", "", "Optional build identifier for trend history")
	fs.IntVar(&opts.run.trendLimit, "trend-limit", 0, "Maximum number of trend history runs to keep")
	fs.DurationVar(&opts.run.timeout, "timeout", 0, "Overall execution timeout for each run")
	fs.BoolVar(&opts.run.githubOutputs, "github-outputs", false, "Write PR-oriented run metrics to $GITHUB_OUTPUT and $GITHUB_STEP_SUMMARY when available")
	fs.BoolVar(&opts.run.githubPRComment, "github-pr-comment", false, "Post the generated PR review body to GitHub using gh")
	fs.IntVar(&opts.run.githubPRNumber, "github-pr-number", 0, "GitHub pull request number to comment on; defaults to GitHub Actions pull_request context when available")
	fs.BoolVar(&opts.run.buildkite.Meta, "buildkite-meta", false, "Write run metrics to Buildkite metadata when buildkite-agent is available")
	fs.BoolVar(&opts.run.buildkite.Annotation, "buildkite-annotation", false, "Write a Buildkite annotation when the run fails and buildkite-agent is available")
	fs.StringVar(&opts.run.gitlab.DotenvPath, "gitlab-dotenv", "", "Write run metrics to a GitLab dotenv report file")
	fs.StringVar(&opts.run.gitlab.AnnotationsPath, "gitlab-annotations", "", "Write a GitLab annotations report JSON file")
	fs.DurationVar(&opts.interval, "interval", time.Second, "Polling interval for file change detection")
	fs.IntVar(&opts.maxRuns, "max-runs", 0, "Optional maximum number of runs before exiting")
	fs.Var(&opts.watchPath, "watch", "Watch path. Repeat to monitor multiple paths; defaults to the resolved config directory")
	if err := fs.Parse(args); err != nil {
		return watchOptions{}, err
	}
	return opts, nil
}

type watchFileState struct {
	Size    int64
	ModTime time.Time
}

func resolveWatchRoots(resolvedConfigPath string, paths []string) []string {
	if len(paths) == 0 {
		return []string{filepath.Dir(resolvedConfigPath)}
	}
	roots := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = resolveConfigRelativePath(resolvedConfigPath, path)
		}
		roots = append(roots, path)
	}
	if len(roots) == 0 {
		return []string{filepath.Dir(resolvedConfigPath)}
	}
	return roots
}

func buildWatchIgnorePaths(resolvedConfigPath string, opts runOptions, cfg cleanr.Config) map[string]struct{} {
	ignore := map[string]struct{}{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !filepath.IsAbs(path) {
			path = resolveConfigRelativePath(resolvedConfigPath, path)
		}
		ignore[filepath.Clean(path)] = struct{}{}
	}
	add(opts.output)
	add(opts.trendFile)
	add(opts.replayArtifactPath)
	add(cfg.Reporting.Output)
	add(cfg.Reporting.TrendFile)
	add(cfg.Reporting.ReplayArtifactFile)
	add(cfg.Governance.Attestation.Output)
	return ignore
}

func collectWatchSnapshot(roots []string, ignorePaths map[string]struct{}) (map[string]watchFileState, error) {
	snapshot := map[string]watchFileState{}
	for _, root := range roots {
		cleanRoot := filepath.Clean(strings.TrimSpace(root))
		if cleanRoot == "" {
			continue
		}
		if err := appendWatchRoot(snapshot, cleanRoot, ignorePaths); err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

func appendWatchRoot(snapshot map[string]watchFileState, root string, ignorePaths map[string]struct{}) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		appendWatchFile(snapshot, root, info, ignorePaths)
		return nil
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return appendWatchWalkEntry(snapshot, path, entry, ignorePaths)
	})
}

func appendWatchFile(snapshot map[string]watchFileState, path string, info os.FileInfo, ignorePaths map[string]struct{}) {
	if shouldIgnoreWatchPath(path, ignorePaths) {
		return
	}
	snapshot[path] = watchFileState{Size: info.Size(), ModTime: info.ModTime().UTC()}
}

func appendWatchWalkEntry(snapshot map[string]watchFileState, path string, entry fs.DirEntry, ignorePaths map[string]struct{}) error {
	cleanPath := filepath.Clean(path)
	if entry.IsDir() {
		if entry.Name() == ".git" {
			return filepath.SkipDir
		}
		return nil
	}
	if shouldIgnoreWatchPath(cleanPath, ignorePaths) {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	snapshot[cleanPath] = watchFileState{Size: info.Size(), ModTime: info.ModTime().UTC()}
	return nil
}

func shouldIgnoreWatchPath(path string, ignorePaths map[string]struct{}) bool {
	if len(ignorePaths) == 0 {
		return false
	}
	_, ok := ignorePaths[filepath.Clean(path)]
	return ok
}

func diffWatchSnapshot(before, after map[string]watchFileState) []string {
	changes := make([]string, 0)
	seen := map[string]struct{}{}
	for path, state := range after {
		prev, ok := before[path]
		if !ok || prev.Size != state.Size || !prev.ModTime.Equal(state.ModTime) {
			changes = append(changes, path)
			seen[path] = struct{}{}
		}
	}
	for path := range before {
		if _, ok := after[path]; ok {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		changes = append(changes, path)
	}
	return changes
}
