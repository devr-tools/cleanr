package types

import (
	"context"
	"time"
)

type Request struct {
	Scenario Scenario
	System   string
	Prompt   string
	Messages []ConversationTurn
	Timeout  time.Duration
	Headers  map[string]string
	Template any
}

type Response struct {
	StatusCode   int
	Body         []byte
	Text         string
	Latency      time.Duration
	Stream       StreamMetrics
	Err          error
	ExtractError error
	Usage        TokenUsage
	Normalized   ProviderResponse
}

type StreamMetrics struct {
	TTFTMS          int64  `json:"ttft_ms,omitempty"`
	DurationMS      int64  `json:"duration_ms,omitempty"`
	ChunkCount      int    `json:"chunk_count,omitempty"`
	ErrorCount      int    `json:"error_count,omitempty"`
	Recovered       bool   `json:"recovered,omitempty"`
	CompletionState string `json:"completion_state,omitempty"`
}

type TokenUsage struct {
	InputTokens  int  `json:"input_tokens,omitempty"`
	OutputTokens int  `json:"output_tokens,omitempty"`
	TotalTokens  int  `json:"total_tokens,omitempty"`
	Heuristic    bool `json:"heuristic,omitempty"`
}

type ProviderResponse struct {
	Provider         string             `json:"provider,omitempty"`
	ID               string             `json:"id,omitempty"`
	Model            string             `json:"model,omitempty"`
	Role             string             `json:"role,omitempty"`
	Status           string             `json:"status,omitempty"`
	FinishReason     string             `json:"finish_reason,omitempty"`
	StopSequence     string             `json:"stop_sequence,omitempty"`
	ToolCalls        []ToolCall         `json:"tool_calls,omitempty"`
	SourceUses       []SourceUse        `json:"source_uses,omitempty"`
	Approvals        []ApprovalArtifact `json:"approvals,omitempty"`
	StateChanges     []StateChange      `json:"state_changes,omitempty"`
	MemoryOperations []MemoryOperation  `json:"memory_operations,omitempty"`
	Raw              map[string]any     `json:"raw,omitempty"`
}

type ToolCall struct {
	ID         string         `json:"id,omitempty"`
	CallID     string         `json:"call_id,omitempty"`
	Type       string         `json:"type,omitempty"`
	Name       string         `json:"name,omitempty"`
	Arguments  string         `json:"arguments,omitempty"`
	ParsedArgs any            `json:"parsed_arguments,omitempty"`
	Input      any            `json:"input,omitempty"`
	Status     string         `json:"status,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

type SourceUse struct {
	ID       string         `json:"id,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Name     string         `json:"name,omitempty"`
	Location string         `json:"location,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type ApprovalArtifact struct {
	ID       string         `json:"id,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Status   string         `json:"status,omitempty"`
	Actor    string         `json:"actor,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Artifact string         `json:"artifact,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type StateChange struct {
	Kind    string         `json:"kind,omitempty"`
	Target  string         `json:"target,omitempty"`
	Action  string         `json:"action,omitempty"`
	Status  string         `json:"status,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Raw     map[string]any `json:"raw,omitempty"`
}

type MemoryOperation struct {
	Action    string         `json:"action,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	Key       string         `json:"key,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Value     string         `json:"value,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type Target interface {
	Invoke(context.Context, Request) Response
}

type Engine interface {
	Name() string
	Run(context.Context, *RunContext) SuiteResult
}

type RunContext struct {
	Config Config
	Target Target
}
