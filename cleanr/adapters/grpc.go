package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"crypto/tls"
	"net"

	"github.com/devr-tools/cleanr/cleanr/core"
	"github.com/golang/protobuf/jsonpb"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type GRPC struct {
	cfg core.TargetConfig
}

func NewGRPC(cfg core.TargetConfig) *GRPC {
	return &GRPC{cfg: cfg}
}

func (t *GRPC) Invoke(ctx context.Context, req core.Request) core.Response {
	payload := buildRequestBody(req, t.cfg)
	data, err := json.Marshal(payload)
	if err != nil {
		return core.Response{Err: err}
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = t.cfg.Timeout()
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := strings.TrimSpace(t.cfg.GRPC.Address)
	conn, err := grpc.DialContext(reqCtx, address,
		grpc.WithTransportCredentials(grpcTransportCredentials(address, t.cfg.GRPC.Plaintext)),
		grpc.WithBlock(),
	)
	if err != nil {
		return core.Response{Err: err}
	}
	defer conn.Close()

	start := time.Now()
	resp, callErr := t.invokeUnary(reqCtx, conn, data, req)
	resp.Latency = time.Since(start)
	if callErr != nil {
		resp.Err = callErr
	}
	return resp
}

func (t *GRPC) invokeUnary(ctx context.Context, conn *grpc.ClientConn, data []byte, req core.Request) (core.Response, error) {
	serviceName, methodName, fullMethod, err := parseGRPCMethod(t.cfg.GRPC.Method)
	if err != nil {
		return core.Response{}, err
	}

	refClient := grpcreflect.NewClientAuto(ctx, conn)
	defer refClient.Reset()

	serviceDesc, err := refClient.ResolveService(serviceName)
	if err != nil {
		return core.Response{Normalized: grpcNormalized(fullMethod, codes.Unimplemented, nil, nil)}, err
	}
	methodDesc := serviceDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		return core.Response{Normalized: grpcNormalized(fullMethod, codes.Unimplemented, nil, nil)}, fmt.Errorf("grpc method %q not found", fullMethod)
	}
	if methodDesc.IsClientStreaming() || methodDesc.IsServerStreaming() {
		return core.Response{Normalized: grpcNormalized(fullMethod, codes.Unimplemented, nil, nil)}, fmt.Errorf("grpc adapter only supports unary methods: %s", fullMethod)
	}

	input := dynamic.NewMessage(methodDesc.GetInputType())
	trimmed := strings.TrimSpace(string(data))
	if trimmed != "" && trimmed != "null" {
		if err := jsonpb.UnmarshalString(trimmed, input); err != nil {
			return core.Response{Normalized: grpcNormalized(fullMethod, codes.InvalidArgument, nil, nil)}, err
		}
	}

	outgoing := grpcMetadata(req.Headers, t.cfg.Headers)
	if len(outgoing) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, outgoing)
	}

	var headers metadata.MD
	var trailers metadata.MD
	stub := grpcdynamic.NewStub(conn)
	msg, err := stub.InvokeRpc(ctx, methodDesc, input, grpc.Header(&headers), grpc.Trailer(&trailers))
	if err != nil {
		code := status.Code(err)
		return core.Response{Normalized: grpcNormalized(fullMethod, code, headers, trailers)}, err
	}

	var body bytes.Buffer
	marshaler := jsonpb.Marshaler{EmitDefaults: true}
	if err := marshaler.Marshal(&body, msg); err != nil {
		return core.Response{Normalized: grpcNormalized(fullMethod, codes.Internal, headers, trailers)}, err
	}

	text := body.String()
	var extractErr error
	if strings.TrimSpace(t.cfg.ResponseField) != "" {
		text, extractErr = extractResponseField(body.Bytes(), t.cfg.ResponseField)
	}

	return core.Response{
		Body:         body.Bytes(),
		Text:         text,
		ExtractError: extractErr,
		Normalized:   grpcNormalized(fullMethod, codes.OK, headers, trailers),
	}, nil
}

// grpcTransportCredentials picks transport credentials for a dial target.
// Auth tokens travel in gRPC headers and every payload is otherwise sent in
// cleartext, so TLS is the default for any remote host. Plaintext is only used
// when the caller explicitly opts in via GRPC.Plaintext or when the target is a
// loopback address (localhost / 127.0.0.1 / ::1), which keeps local test
// targets working without exposing traffic on the network.
func grpcTransportCredentials(address string, plaintext bool) credentials.TransportCredentials {
	if plaintext || isLoopbackGRPCAddress(address) {
		return insecure.NewCredentials()
	}
	return credentials.NewTLS(&tls.Config{})
}

// isLoopbackGRPCAddress reports whether a gRPC dial target refers to the local
// host. It accepts host, host:port, and bare-port forms.
func isLoopbackGRPCAddress(address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}
	// Strip a gRPC name-resolver scheme such as "dns:///" or "passthrough:///".
	if idx := strings.Index(address, "://"); idx >= 0 {
		address = address[idx+3:]
	}
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func parseGRPCMethod(raw string) (string, string, string, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if strings.Contains(trimmed, "/") {
		parts := strings.Split(trimmed, "/")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return "", "", "", fmt.Errorf("invalid grpc method %q", raw)
		}
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), trimmed, nil
	}
	idx := strings.LastIndex(trimmed, ".")
	if idx <= 0 || idx == len(trimmed)-1 {
		return "", "", "", fmt.Errorf("invalid grpc method %q", raw)
	}
	service := strings.TrimSpace(trimmed[:idx])
	method := strings.TrimSpace(trimmed[idx+1:])
	return service, method, service + "/" + method, nil
}

func grpcMetadata(lists ...map[string]string) metadata.MD {
	md := metadata.MD{}
	for _, values := range lists {
		for key, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			md.Append(strings.ToLower(strings.TrimSpace(key)), value)
		}
	}
	return md
}

func grpcNormalized(fullMethod string, code codes.Code, headers, trailers metadata.MD) core.ProviderResponse {
	raw := map[string]any{
		"grpc_method": fullMethod,
		"grpc_code":   code.String(),
	}
	if len(headers) > 0 {
		raw["headers"] = map[string][]string(headers)
	}
	if len(trailers) > 0 {
		raw["trailers"] = map[string][]string(trailers)
	}
	return core.ProviderResponse{
		Provider: "grpc",
		Status:   code.String(),
		Raw:      raw,
	}
}
