package devtools

import "context"

// CodeGuard is kept for compatibility with older callers.
// Deprecated: use PackageCodeGuard instead.
func (r Runner) CodeGuard(ctx context.Context, opts CIOptions) error {
	return r.PackageCodeGuard(ctx, opts)
}
