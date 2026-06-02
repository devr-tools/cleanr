package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/cleanr/cleanr"
)

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedStringFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*f = append(*f, value)
	return nil
}

type datasetReviewGateOptions struct {
	FailOnPending  bool
	FailOnRejected bool
	MinApproved    int
	MaxDuplicates  int
	WriteGHOutputs bool
}

type buildkiteOptions struct {
	Meta            bool
	Annotation      bool
	UploadArtifacts bool
}

type datasetReviewGateResult struct {
	Passed   bool
	Messages []string
}

type datasetReviewCommandOptions struct {
	Input         string
	Policy        string
	BaseConfig    string
	Profile       string
	Output        string
	MergeOutput   string
	Format        string
	MergeInPlace  bool
	GitHubOutputs bool
	Buildkite     buildkiteOptions
	Gate          datasetReviewGateOptions
	Approve       []string
	Reject        []string
	PromoteStable []string
	PromoteReg    []string
	AddTag        []string
	SetTags       []string
	SetMetadata   []string
}

type datasetReviewCommandContext struct {
	BaseConfig cleanr.Config
	Reviewed   cleanr.ReviewedScenarioDataset
	PolicyPath string
	OutputPath string
	MergePath  string
	JSONOutput bool
}

func datasetCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "dataset error: expected one of export or import")
		return 2
	}
	switch args[0] {
	case "export":
		return datasetExportCmd(args[1:], stdout, stderr)
	case "import":
		return datasetImportCmd(args[1:], stdout, stderr)
	case "review":
		return datasetReviewCmd(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "dataset error: unsupported subcommand %s\n", args[0])
		return 2
	}
}

func datasetExportCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dataset export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to cleanr config")
	profile := fs.String("profile", "", "Optional staged config profile: pr, main, or release")
	replayPath := fs.String("replay-artifact", "", "Path to replay artifact file")
	output := fs.String("output", "cleanr.dataset.yaml", "Path to write the exported scenario dataset")
	includeAll := fs.Bool("all", false, "Include all scenarios instead of only reviewed replay failures")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	cfg, err := cleanr.LoadConfigFile(resolvedConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	artifactPath := strings.TrimSpace(*replayPath)
	if artifactPath == "" {
		artifactPath = cfg.Reporting.ReplayArtifactFile
	}
	artifactPath = resolveConfigRelativePath(resolvedConfigPath, artifactPath)
	if strings.TrimSpace(artifactPath) == "" {
		_, _ = fmt.Fprintln(stderr, "dataset export error: no replay artifact configured; pass -replay-artifact or set reporting.replay_artifact_file")
		return 2
	}
	artifact, err := cleanr.LoadReplayArtifactFile(artifactPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	dataset := cleanr.ExportScenarioDataset(cfg, artifact, *includeAll)
	outputPath := resolveConfigRelativePath(resolvedConfigPath, *output)
	if err := cleanr.WriteScenarioDatasetFile(outputPath, dataset); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset export error: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote %d scenarios to %s\n", len(dataset.Scenarios), outputPath)
	return 0
}

func datasetImportCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dataset import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("input", "", "Path to scenario dataset file")
	baseConfig := fs.String("base-config", "", "Optional base cleanr config to merge into")
	output := fs.String("output", "cleanr.imported.yaml", "Path to write the merged cleanr config")
	approveGenerated := fs.Bool("approve-generated", false, "Allow importing generated datasets that require explicit review")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*input) == "" {
		_, _ = fmt.Fprintln(stderr, "dataset import error: -input is required")
		return 2
	}

	dataset, err := cleanr.LoadScenarioDatasetFile(*input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset import error: %v\n", err)
		return 2
	}
	if len(dataset.Scenarios) == 0 {
		_, _ = fmt.Fprintln(stderr, "dataset import error: dataset contains no scenarios")
		return 2
	}
	if dataset.Source == "cleanr-generation" && dataset.ReviewRequired && !*approveGenerated {
		_, _ = fmt.Fprintln(stderr, "dataset import error: generated dataset requires explicit review; rerun with -approve-generated after review")
		return 2
	}

	cfg := cleanr.ExampleConfig()
	basePath := strings.TrimSpace(*baseConfig)
	if basePath != "" {
		cfg, err = cleanr.LoadConfigFile(basePath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "dataset import error: %v\n", err)
			return 2
		}
	}
	cfg = cleanr.MergeDatasetIntoConfig(cfg, dataset)
	if err := cleanr.WriteConfigFile(*output, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "dataset import error: %v\n", err)
		return 2
	}
	_, _ = fmt.Fprintf(stdout, "wrote merged config with %d scenarios to %s\n", len(cfg.Scenarios), *output)
	return 0
}
