package mcpserver

import (
	"encoding/json"
	"fmt"
	"io"
)

const protocolVersion = "2025-06-18"

const (
	jsonRPCParseError     = -32700
	jsonRPCInvalidRequest = -32600
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
	jsonRPCInternalError  = -32603
	jsonRPCServerError    = -32000
)

type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type responseEnvelope struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *errorEnvelope `json:"error,omitempty"`
}

type errorEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      map[string]any `json:"clientInfo"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func successResponse(id any, result any) *responseEnvelope {
	return &responseEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func errorResponse(id any, code int, message string, data any) *responseEnvelope {
	return &responseEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Error: &errorEnvelope{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func writeMessage(w io.Writer, msg *responseEnvelope) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
