package integrations

import (
	"context"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type nativeSinkPublisher interface {
	supports(core.ResultSinkConfig) bool
	publish(context.Context, core.ResultSinkConfig, SinkPayload) (string, error)
}

var nativeSinkPublishers = []nativeSinkPublisher{
	braintrustSinkPublisher{},
	langfuseSinkPublisher{},
	postHogSinkPublisher{},
}

func buildSinkPayload(sink core.ResultSinkConfig, report core.Report, replay *core.ReplayArtifact, attestation *core.ReleaseGateAttestation) SinkPayload {
	payload := SinkPayload{
		Version:          "v1alpha1",
		Source:           "cleanr",
		SinkType:         strings.TrimSpace(sink.Type),
		Project:          strings.TrimSpace(sink.Project),
		Experiment:       strings.TrimSpace(sink.Experiment),
		Target:           report.Name,
		BuildID:          buildID(report.Metadata),
		GeneratedAt:      report.GeneratedAt,
		LocalBlocking:    true,
		RemoteBestEffort: true,
		Report:           report,
	}
	if sink.IncludeReplay && replay != nil {
		payload.ReplayArtifact = replay
	}
	if sink.IncludeAttest && attestation != nil {
		payload.Attestation = attestation
	}
	return payload
}

func publishNativeSink(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, bool, error) {
	for _, publisher := range nativeSinkPublishers {
		if !publisher.supports(sink) {
			continue
		}
		runURL, err := publisher.publish(ctx, sink, payload)
		return runURL, true, err
	}
	return "", false, nil
}

type braintrustSinkPublisher struct{}

func (braintrustSinkPublisher) supports(sink core.ResultSinkConfig) bool {
	return useNativeBraintrustSink(sink)
}

func (braintrustSinkPublisher) publish(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	return postBraintrustSinkPayload(ctx, sink, payload)
}

type langfuseSinkPublisher struct{}

func (langfuseSinkPublisher) supports(sink core.ResultSinkConfig) bool {
	return useNativeLangfuseSink(sink)
}

func (langfuseSinkPublisher) publish(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	return postLangfuseSinkPayload(ctx, sink, payload)
}

type postHogSinkPublisher struct{}

func (postHogSinkPublisher) supports(sink core.ResultSinkConfig) bool {
	return useNativePostHogSink(sink)
}

func (postHogSinkPublisher) publish(ctx context.Context, sink core.ResultSinkConfig, payload SinkPayload) (string, error) {
	return postPostHogSinkPayload(ctx, sink, payload)
}
