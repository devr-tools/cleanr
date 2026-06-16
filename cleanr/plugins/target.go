package plugins

import (
	"context"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type Target struct {
	base      core.Target
	manifests []core.PluginManifest
}

func WrapTarget(base core.Target, manifests []core.PluginManifest) core.Target {
	if len(manifests) == 0 {
		return base
	}
	if !NewRegistry(manifests).HasStateAdapters() {
		return base
	}
	return Target{base: base, manifests: manifests}
}

func (t Target) Invoke(ctx context.Context, req core.Request) core.Response {
	resp := t.base.Invoke(ctx, req)
	if resp.Err != nil {
		return resp
	}
	adapted, err := ApplyStateAdapters(ctx, req, resp, t.manifests)
	if err != nil {
		resp.Err = err
		return resp
	}
	return adapted
}
