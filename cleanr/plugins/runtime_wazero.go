package plugins

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func runWASMModule(ctx context.Context, entry Entry, input []byte) (bytes.Buffer, error) {
	modulePath := strings.TrimSpace(entry.Resolved)
	if modulePath == "" {
		return bytes.Buffer{}, fmt.Errorf("missing wasm module path")
	}
	wasm, err := os.ReadFile(modulePath)
	if err != nil {
		return bytes.Buffer{}, err
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return bytes.Buffer{}, fmt.Errorf("instantiate wasi: %w", err)
	}
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return bytes.Buffer{}, fmt.Errorf("compile wasm module: %w", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	config := wazero.NewModuleConfig().
		WithStdin(bytes.NewReader(input)).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs(entry.Args...).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep()
	for _, item := range buildEntryEnv(entry) {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		config = config.WithEnv(key, value)
	}

	module, err := runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return bytes.Buffer{}, fmt.Errorf("%s", msg)
	}
	defer module.Close(ctx)

	if entrypoint := strings.TrimSpace(entry.Runtime.Entrypoint); entrypoint != "" && entrypoint != "_start" {
		fn := module.ExportedFunction(entrypoint)
		if fn == nil {
			return bytes.Buffer{}, fmt.Errorf("wasm entrypoint %q not found", entrypoint)
		}
		if _, err := fn.Call(ctx); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return bytes.Buffer{}, fmt.Errorf("%s", msg)
		}
	}

	return stdout, nil
}
