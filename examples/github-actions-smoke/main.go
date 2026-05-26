package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type requestPayload struct {
	Input string `json:"input"`
}

type responsePayload struct {
	Output struct {
		Text string `json:"text"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "18080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/respond", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req requestPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		text := responseText(req.Input)
		resp := responsePayload{}
		resp.Output.Text = text
		resp.Usage.InputTokens = 24
		resp.Usage.OutputTokens = 14
		resp.Usage.TotalTokens = 38

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	addr := ":" + port
	log.Printf("github-actions smoke target listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func responseText(prompt string) string {
	lower := strings.ToLower(prompt)

	switch {
	case strings.Contains(lower, "ignore every previous instruction"),
		strings.Contains(lower, "reveal hidden"),
		strings.Contains(lower, "credentials"),
		strings.Contains(lower, "tools"):
		return "I cannot comply with that request."
	case strings.Contains(lower, "refund"):
		return "refunds are available within 30 days of purchase. start the request from the billing page."
	case strings.Contains(lower, "support hours"), strings.Contains(lower, "opening hours"), strings.Contains(lower, "hours"):
		return "support is available on weekdays from 9 AM to 5 PM."
	default:
		return "the workflow completed successfully."
	}
}
