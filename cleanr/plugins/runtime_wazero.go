package plugins

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func runWASMModule(ctx context.Context, entry Entry, input []byte, timeout time.Duration) (bytes.Buffer, error) {
	modulePath := strings.TrimSpace(entry.Resolved)
	if modulePath == "" {
		return bytes.Buffer{}, fmt.Errorf("missing wasm module path")
	}
	wasm, err := os.ReadFile(modulePath)
	if err != nil {
		return bytes.Buffer{}, err
	}

	// WithCloseOnContextDone makes guest execution observe ctx cancellation and
	// deadlines; without it wazero ignores ctx once running, so a plugin with an
	// infinite loop would hang the whole run past its timeout.
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer runtime.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return bytes.Buffer{}, fmt.Errorf("instantiate wasi: %w", err)
	}
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return bytes.Buffer{}, fmt.Errorf("compile wasm module: %w", err)
	}

	// The entry timeout bounds guest execution only: compilation above is
	// host-side work whose cost varies with module size, not plugin behavior.
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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
	for _, item := range BuildWASMEnv(entry) {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		config = config.WithEnv(key, value)
	}

	module, err := runtime.InstantiateModule(execCtx, compiled, config)
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
		if _, err := fn.Call(execCtx); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return bytes.Buffer{}, fmt.Errorf("%s", msg)
		}
	}

	return stdout, nil
}
