package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

// Usage holds token counts from a completed stream.
type Usage struct {
	Model        string
	InputTokens  int
	OutputTokens int
}

// StreamChat streams a chat response from Ollama as SSE events.
// Returns usage stats after completion.
func StreamChat(w http.ResponseWriter, r *http.Request, req ChatRequest) Usage {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return Usage{}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		writeSSEError(w, flusher, "Failed to encode request")
		return Usage{}
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(BaseURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		writeSSEError(w, flusher, "Ollama unavailable")
		return Usage{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeSSEError(w, flusher, fmt.Sprintf("Ollama returned %d", resp.StatusCode))
		return Usage{}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	start := time.Now()
	tokenCount := 0
	var modelName string
	var usage Usage

	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return usage
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		modelName = chunk.Model

		if chunk.Done {
			elapsed := time.Since(start).Seconds()
			tokPerSec := float64(0)
			if chunk.EvalDuration > 0 {
				tokPerSec = float64(chunk.EvalCount) / (float64(chunk.EvalDuration) / 1e9)
			} else if elapsed > 0 {
				tokPerSec = float64(tokenCount) / elapsed
			}

			status := CheckStatus()
			backend := "CPU"
			if status.GPUOnline {
				backend = "GPU"
			}

			meta := fmt.Sprintf(
				`<span style="color:var(--term-fg-dim);font-size:13px">[%s %s %.1f tok/s %d tokens %.1fs]</span>`,
				html.EscapeString(modelName),
				backend,
				tokPerSec,
				chunk.EvalCount,
				elapsed,
			)
			fmt.Fprintf(w, "event: done\ndata: %s\n\n", meta)
			flusher.Flush()

			usage = Usage{
				Model:        modelName,
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
			}
			return usage
		}

		if chunk.Message.Content != "" {
			tokenCount++
			tokenJSON, _ := json.Marshal(chunk.Message.Content)
			fmt.Fprintf(w, "event: token\ndata: %s\n\n", tokenJSON)
			flusher.Flush()
		}
	}
	return usage
}

// StreamGenerate streams a generate response (non-chat) as SSE events.
func StreamGenerate(w http.ResponseWriter, r *http.Request, model, prompt, system string) Usage {
	messages := []ChatMessage{{Role: "user", Content: prompt}}
	if system != "" {
		messages = append([]ChatMessage{{Role: "system", Content: system}}, messages...)
	}
	return StreamChat(w, r, ChatRequest{Model: model, Messages: messages})
}

func writeSSEError(w http.ResponseWriter, flusher http.Flusher, msg string) {
	errHTML := fmt.Sprintf(`<div class="text-red-400 text-sm mt-2">Error: %s</div>`, html.EscapeString(msg))
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", errHTML)
	flusher.Flush()
}
