package integrations

import (
	"fmt"
	"strings"
)

func applyConfigPatchOperation(root map[string]any, op BraintrustConfigPatchOperation) error {
	switch strings.TrimSpace(op.Op) {
	case "set":
		return applySetPatch(root, strings.TrimSpace(op.Path), op.Value)
	case "append_unique":
		return applyAppendUniquePatch(root, strings.TrimSpace(op.Path), op.Value)
	default:
		return fmt.Errorf("apply config patch: unsupported op %q for %s", op.Op, op.Path)
	}
}

func applySetPatch(root map[string]any, path string, value any) error {
	parent, leaf, err := resolvePatchParent(root, path)
	if err != nil {
		return err
	}
	parent[leaf] = value
	return nil
}

func applyAppendUniquePatch(root map[string]any, path string, value any) error {
	parent, leaf, err := resolvePatchParent(root, path)
	if err != nil {
		return err
	}
	var existing []string
	if raw, ok := parent[leaf]; ok {
		items, err := toStringSlice(raw)
		if err != nil {
			return fmt.Errorf("apply config patch %s: %w", path, err)
		}
		existing = items
	}
	additions, err := toStringSlice(value)
	if err != nil {
		return fmt.Errorf("apply config patch %s: %w", path, err)
	}
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		seen[item] = struct{}{}
	}
	for _, item := range additions {
		if _, ok := seen[item]; ok {
			continue
		}
		existing = append(existing, item)
		seen[item] = struct{}{}
	}
	parent[leaf] = existing
	return nil
}

func resolvePatchParent(root map[string]any, path string) (map[string]any, string, error) {
	segments := strings.Split(path, ".")
	if len(segments) == 0 || strings.TrimSpace(path) == "" {
		return nil, "", fmt.Errorf("apply config patch: empty path")
	}
	current := root
	for _, segment := range segments[:len(segments)-1] {
		next, err := descendPatchSegment(current, segment)
		if err != nil {
			return nil, "", fmt.Errorf("apply config patch %s: %w", path, err)
		}
		current = next
	}
	return current, segments[len(segments)-1], nil
}

func descendPatchSegment(current map[string]any, segment string) (map[string]any, error) {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return nil, fmt.Errorf("invalid empty path segment")
	}
	if !strings.Contains(segment, "[") {
		child, ok := current[segment]
		if !ok {
			next := map[string]any{}
			current[segment] = next
			return next, nil
		}
		mapped, ok := child.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("segment %s is not an object", segment)
		}
		return mapped, nil
	}

	open := strings.Index(segment, "[")
	close := strings.LastIndex(segment, "]")
	if open <= 0 || close <= open+1 {
		return nil, fmt.Errorf("invalid selector segment %s", segment)
	}
	key := strings.TrimSpace(segment[:open])
	selector := strings.TrimSpace(segment[open+1 : close])
	parts := strings.SplitN(selector, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid selector segment %s", segment)
	}
	field := strings.TrimSpace(parts[0])
	want := strings.TrimSpace(parts[1])
	items, ok := current[key]
	if !ok {
		return nil, fmt.Errorf("segment %s does not exist", key)
	}
	list, ok := items.([]any)
	if !ok {
		return nil, fmt.Errorf("segment %s is not a list", key)
	}
	for _, item := range list {
		mapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(mapped[field]) == want {
			return mapped, nil
		}
	}
	return nil, fmt.Errorf("no list item in %s matched %s=%s", key, field, want)
}

func toStringSlice(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, fmt.Sprint(item))
		}
		return out, nil
	case string:
		return []string{typed}, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("expected a string or string list")
	}
}
