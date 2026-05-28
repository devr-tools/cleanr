package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type BraintrustSyncArtifacts struct {
	ExperimentID      string
	ExperimentName    string
	ExperimentURL     string
	ExperimentCreated time.Time
	ReplayArtifact    *core.ReplayArtifact
	InsightPayload    map[string]any
}

func LoadBraintrustSyncArtifacts(ctx context.Context, source core.TrendSourceConfig) (BraintrustSyncArtifacts, error) {
	client := newBraintrustClient(source.BaseURL, source.URL, source.APIKeyEnv, source.Headers, source.TimeoutMS)
	family := strings.TrimSpace(source.Experiment)
	limit := source.HistoryLimit
	if limit <= 0 {
		limit = 10
	}
	experiments, err := client.listExperiments(ctx, source.APIKeyEnv, source.Project, family, limit)
	if err != nil {
		return BraintrustSyncArtifacts{}, err
	}
	for _, experiment := range experiments {
		artifact, err := client.fetchReplayArtifact(ctx, experiment.ID)
		if err != nil {
			return BraintrustSyncArtifacts{}, err
		}
		insight, err := client.fetchSyncInsightPayload(ctx, experiment.ID)
		if err != nil {
			return BraintrustSyncArtifacts{}, err
		}
		if artifact == nil && len(insight) == 0 {
			continue
		}
		summaryURL := ""
		var summary braintrustExperimentSummary
		if err := client.http.getJSON(ctx, path.Join("/v1/experiment", experiment.ID, "summarize"), nil, &summary); err == nil {
			summaryURL = strings.TrimSpace(summary.ExperimentURL)
		}
		return BraintrustSyncArtifacts{
			ExperimentID:      experiment.ID,
			ExperimentName:    experiment.Name,
			ExperimentURL:     summaryURL,
			ExperimentCreated: experiment.Created.UTC(),
			ReplayArtifact:    artifact,
			InsightPayload:    insight,
		}, nil
	}
	return BraintrustSyncArtifacts{}, fmt.Errorf("load braintrust sync artifacts %s: no replay artifact or sync insight found", displayName(source.Project, "braintrust"))
}

func (c *braintrustClient) fetchReplayArtifact(ctx context.Context, experimentID string) (*core.ReplayArtifact, error) {
	query := fmt.Sprintf(
		"SELECT output.replay_artifact AS replay_artifact FROM experiment('%s') WHERE metadata.cleanr.record_type = 'run' AND output.replay_artifact IS NOT NULL LIMIT 1",
		experimentID,
	)
	var resp braintrustBTQLResponse
	if err := c.http.postJSON(ctx, "/btql", map[string]any{
		"query": query,
		"fmt":   "json",
	}, &resp); err != nil {
		return nil, fmt.Errorf("load braintrust sync artifacts: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0]["replay_artifact"] == nil {
		return nil, nil
	}
	raw, err := json.Marshal(resp.Data[0]["replay_artifact"])
	if err != nil {
		return nil, fmt.Errorf("load braintrust sync artifacts: %w", err)
	}
	var artifact core.ReplayArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return nil, fmt.Errorf("load braintrust sync artifacts: %w", err)
	}
	return &artifact, nil
}

func (c *braintrustClient) fetchSyncInsightPayload(ctx context.Context, experimentID string) (map[string]any, error) {
	query := fmt.Sprintf(
		"SELECT output.cleanr_sync AS cleanr_sync FROM experiment('%s') WHERE metadata.cleanr.record_type = 'sync_insight' AND output.cleanr_sync IS NOT NULL LIMIT 1",
		experimentID,
	)
	var resp braintrustBTQLResponse
	if err := c.http.postJSON(ctx, "/btql", map[string]any{
		"query": query,
		"fmt":   "json",
	}, &resp); err != nil {
		return nil, fmt.Errorf("load braintrust sync artifacts: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0]["cleanr_sync"] == nil {
		return nil, nil
	}
	payload, ok := resp.Data[0]["cleanr_sync"].(map[string]any)
	if ok {
		return payload, nil
	}
	raw, err := json.Marshal(resp.Data[0]["cleanr_sync"])
	if err != nil {
		return nil, fmt.Errorf("load braintrust sync artifacts: %w", err)
	}
	var payloadMap map[string]any
	if err := json.Unmarshal(raw, &payloadMap); err != nil {
		return nil, fmt.Errorf("load braintrust sync artifacts: %w", err)
	}
	return payloadMap, nil
}
