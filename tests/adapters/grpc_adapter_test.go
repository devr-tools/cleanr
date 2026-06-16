package tests

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/devr-tools/cleanr/cleanr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

func TestGRPCTargetInvokeRendersPayloadExtractsResponseAndCapturesMetadata(t *testing.T) {
	t.Parallel()

	addr := startHealthGRPCServer(t, grpcHealthServer{})
	target := cleanr.NewGRPCTarget(cleanr.TargetConfig{
		Type:          "grpc",
		ResponseField: "status",
		Headers: map[string]string{
			"x-base": "base-header",
		},
		GRPC: cleanr.GRPCConfig{
			Address:   addr,
			Method:    "/grpc.health.v1.Health/Check",
			Plaintext: true,
		},
		RequestTemplate: map[string]any{
			"service": "{{prompt}}",
		},
	})

	req := cleanr.Request{
		Scenario: cleanr.Scenario{Name: "grpc-health"},
		Prompt:   "catalog-service",
		Headers: map[string]string{
			"x-request": "request-header",
		},
		Timeout: time.Second,
	}

	resp := target.Invoke(context.Background(), req)
	if resp.Err != nil || resp.ExtractError != nil {
		t.Fatalf("unexpected grpc response errors: err=%v extract=%v", resp.Err, resp.ExtractError)
	}
	if resp.Text != "SERVING" {
		t.Fatalf("unexpected grpc response text: %q", resp.Text)
	}
	if resp.Normalized.Provider != "grpc" || resp.Normalized.Status != codes.OK.String() {
		t.Fatalf("unexpected normalized grpc response: %+v", resp.Normalized)
	}
	if resp.Normalized.Raw["grpc_method"] != "grpc.health.v1.Health/Check" {
		t.Fatalf("expected grpc method in raw payload, got %+v", resp.Normalized.Raw)
	}

	headers, ok := resp.Normalized.Raw["headers"].(map[string][]string)
	if !ok {
		t.Fatalf("expected response headers in raw payload, got %+v", resp.Normalized.Raw)
	}
	if headers["x-served-by"][0] != "grpc-test" {
		t.Fatalf("unexpected grpc response headers: %+v", headers)
	}

	trailers, ok := resp.Normalized.Raw["trailers"].(map[string][]string)
	if !ok {
		t.Fatalf("expected response trailers in raw payload, got %+v", resp.Normalized.Raw)
	}
	if trailers["x-trace-id"][0] != "trace-grpc-1" {
		t.Fatalf("unexpected grpc response trailers: %+v", trailers)
	}
}

func TestGRPCTargetInvokeNormalizesUnaryErrors(t *testing.T) {
	t.Parallel()

	addr := startHealthGRPCServer(t, grpcHealthServer{})
	target := cleanr.NewGRPCTarget(cleanr.TargetConfig{
		Type: "grpc",
		Headers: map[string]string{
			"x-base": "base-header",
		},
		GRPC: cleanr.GRPCConfig{
			Address: addr,
			Method:  "grpc.health.v1.Health/Check",
		},
		RequestTemplate: map[string]any{
			"service": "{{prompt}}",
		},
	})

	resp := target.Invoke(context.Background(), cleanr.Request{
		Prompt: "missing-service",
		Headers: map[string]string{
			"x-request": "request-header",
		},
		Timeout: time.Second,
	})
	if resp.Err == nil {
		t.Fatal("expected grpc error")
	}
	if status.Code(resp.Err) != codes.NotFound {
		t.Fatalf("unexpected grpc error code: %v", status.Code(resp.Err))
	}
	if resp.Normalized.Status != codes.NotFound.String() {
		t.Fatalf("unexpected normalized grpc status: %+v", resp.Normalized)
	}
}

type grpcHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (grpcHealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	base := md.Get("x-base")
	if len(base) != 1 || base[0] != "base-header" {
		return nil, status.Error(codes.InvalidArgument, "missing x-base metadata")
	}
	request := md.Get("x-request")
	if len(request) != 1 || request[0] != "request-header" {
		return nil, status.Error(codes.InvalidArgument, "missing x-request metadata")
	}
	if req.GetService() == "missing-service" {
		return nil, status.Error(codes.NotFound, "service not found")
	}
	if req.GetService() != "catalog-service" {
		return nil, status.Error(codes.InvalidArgument, "unexpected service payload")
	}
	if err := grpc.SendHeader(ctx, metadata.Pairs("x-served-by", "grpc-test")); err != nil {
		return nil, err
	}
	if err := grpc.SetTrailer(ctx, metadata.Pairs("x-trace-id", "trace-grpc-1")); err != nil {
		return nil, err
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func startHealthGRPCServer(t *testing.T, svc grpcHealthServer) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(server, svc)
	reflection.Register(server)

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Logf("grpc server stopped: %v", err)
		}
	}()

	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	return listener.Addr().String()
}
