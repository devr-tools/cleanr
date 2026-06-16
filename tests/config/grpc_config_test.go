package tests

import (
	"testing"

	"github.com/devr-tools/cleanr/cleanr"
)

func TestValidateConfigGRPCTargetRequiresAddress(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Target.Type = "grpc"
	cfg.Target.URL = ""
	cfg.Target.PromptField = ""
	cfg.Target.ResponseField = ""
	cfg.Target.GRPC = cleanr.GRPCConfig{
		Method: "grpc.health.v1.Health/Check",
	}

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected grpc validation error")
	}
	want := "invalid config: target.grpc.address: is required. Fix: set the gRPC server address, for example 127.0.0.1:50051"
	if err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}

func TestValidateConfigGRPCTargetRequiresMethod(t *testing.T) {
	t.Parallel()

	cfg := cleanr.ExampleConfig()
	cfg.Target.Type = "grpc"
	cfg.Target.URL = ""
	cfg.Target.PromptField = ""
	cfg.Target.ResponseField = ""
	cfg.Target.GRPC = cleanr.GRPCConfig{
		Address: "127.0.0.1:50051",
	}

	err := cleanr.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected grpc validation error")
	}
	want := "invalid config: target.grpc.method: is required. Fix: set the fully-qualified gRPC method such as grpc.testing.TestService/UnaryCall"
	if err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}
