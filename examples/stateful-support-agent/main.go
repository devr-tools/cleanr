package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type request struct {
	System string `json:"system"`
	Input  string `json:"input"`
}

type caseState struct {
	CaseID            string `json:"case_id"`
	CustomerID        string `json:"customer_id"`
	PasswordReset     bool   `json:"password_reset"`
	ConfirmationDraft bool   `json:"confirmation_draft"`
	Status            string `json:"status"`
	UpdatedAt         string `json:"updated_at"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/chat", handleChat)

	addr := ":8091"
	log.Printf("stateful support agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	state, err := runWorkflow(req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"output": map[string]any{
			"text": "I verified the customer, reset the password, updated the case, and drafted the confirmation email for review.",
		},
		"trace": map[string]any{
			"provider":      "stateful-support-agent",
			"model":         "workflow-v1",
			"finish_reason": "stop",
			"tool_calls": []any{
				map[string]any{"name": "lookup_customer", "arguments": `{"customer_id":"cust_123"}`},
				map[string]any{"name": "run_sql", "arguments": `{"query":"SELECT id, status FROM support_cases WHERE id = 'case-123'"}`},
				map[string]any{"name": "draft_email", "arguments": `{"to":"customer@example.com","body":"Password reset complete. Review before sending."}`},
			},
			"approvals": []any{
				map[string]any{"id": "approval-1", "status": "approved", "actor": "supervisor", "artifact": "ticket://approval-1"},
			},
			"state_changes": []any{
				map[string]any{"kind": "ticket", "action": "update", "target": state.CaseID, "status": "applied", "summary": "marked password reset complete"},
				map[string]any{"kind": "email", "action": "draft", "target": "customer@example.com", "status": "applied", "summary": "drafted confirmation email"},
			},
		},
		"usage": map[string]any{
			"input_tokens":  42,
			"output_tokens": 28,
			"total_tokens":  70,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func runWorkflow(input string) (caseState, error) {
	stateDir := filepath.Join("examples", "stateful-support-agent", "data")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return caseState{}, err
	}

	state := caseState{
		CaseID:            "case-123",
		CustomerID:        "cust_123",
		PasswordReset:     true,
		ConfirmationDraft: true,
		Status:            "pending-review",
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	if !strings.Contains(strings.ToLower(input), "reset the password") {
		state.Status = "no-op"
		state.PasswordReset = false
		state.ConfirmationDraft = false
	}

	path := filepath.Join(stateDir, "case-123.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return caseState{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return caseState{}, err
	}
	return state, nil
}
