package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/iryzhkov/cv-site/middleware"
	"github.com/iryzhkov/cv-site/ollama"
)

var benchmarkPrompts = []struct {
	Name   string
	Prompt string
}{
	{"Short Q&A", "What is the capital of France? Answer in one sentence."},
	{"Code Generation", "Write a Go function that checks if a string is a palindrome. Include the function signature and body only."},
	{"Reasoning", "A farmer has 17 sheep. All but 9 die. How many sheep are left? Explain your reasoning step by step."},
	{"Summarization", "Explain the transformer architecture in exactly 3 bullet points, each one sentence long."},
}

type BenchmarkResult struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	TokPerSec   float64 `json:"tok_per_sec"`
	TotalTokens int     `json:"total_tokens"`
	TotalTimeMs int64   `json:"total_time_ms"`
	TTFT_Ms     int64   `json:"ttft_ms"`
}

func Benchmarks(w http.ResponseWriter, r *http.Request) {
	Templates["benchmarks.html"].ExecuteTemplate(w, "base", map[string]any{
		"Prompts": benchmarkPrompts,
		"Active":  "benchmarks",
		"HasLive": middleware.HasLiveAccess(r),
	})
}

// RunBenchmark handles POST /api/benchmark — runs a single benchmark and returns results as HTML.
func RunBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	model := r.FormValue("model")
	prompt := r.FormValue("prompt")
	promptName := r.FormValue("prompt_name")

	if model == "" || prompt == "" {
		http.Error(w, "model and prompt required", http.StatusBadRequest)
		return
	}

	// Restrict to small model for external users without access token
	if !middleware.HasLiveAccess(r) {
		model = RestrictedModel
	}

	chatReq := ollama.ChatRequest{
		Model:    model,
		Messages: []ollama.ChatMessage{{Role: "user", Content: prompt}},
		Stream:   false,
	}

	body, _ := json.Marshal(chatReq)

	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(ollama.BaseURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="text-red-400 text-sm">Error: %s</div>`, html.EscapeString(err.Error()))
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	totalTime := time.Since(start)

	var chunk ollama.StreamChunk
	json.Unmarshal(respBody, &chunk)

	tokPerSec := float64(0)
	if chunk.EvalDuration > 0 {
		tokPerSec = float64(chunk.EvalCount) / (float64(chunk.EvalDuration) / 1e9)
	}

	result := BenchmarkResult{
		Model:       model,
		Prompt:      promptName,
		TokPerSec:   tokPerSec,
		TotalTokens: chunk.EvalCount,
		TotalTimeMs: totalTime.Milliseconds(),
	}

	w.Header().Set("Content-Type", "text/html")
	Templates["benchmark-result.html"].ExecuteTemplate(w, "benchmark-result", result)
}
