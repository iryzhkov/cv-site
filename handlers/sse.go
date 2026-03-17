package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/iryzhkov/cv-site/middleware"
	"github.com/iryzhkov/cv-site/ollama"
)

// RestrictedModel is the small model used for external users without an access token.
const RestrictedModel = "qwen3:8b"

type pendingStream struct {
	Request   ollama.ChatRequest
	Company   string
	SessionID string
}

// pendingStreams stores pending chat requests keyed by stream ID.
var pendingStreams = struct {
	sync.RWMutex
	m map[string]pendingStream
}{m: make(map[string]pendingStream)}

func newStreamID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ChatSubmit handles POST /api/chat — stores the request and returns
// an HTML fragment that initiates SSE streaming.
func ChatSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompt := r.FormValue("prompt")
	model := r.FormValue("model")
	system := r.FormValue("system")

	if prompt == "" {
		http.Error(w, "prompt required", http.StatusBadRequest)
		return
	}
	if model == "" {
		model = "gemma3:12b"
	}

	// Restrict to small model for external users without access token
	if !middleware.HasLiveAccess(r) {
		model = RestrictedModel
	}

	messages := []ollama.ChatMessage{}
	if system != "" {
		messages = append(messages, ollama.ChatMessage{Role: "system", Content: system})
	}

	// Parse conversation history if provided
	historyJSON := r.FormValue("history")
	if historyJSON != "" {
		var history []ollama.ChatMessage
		if err := json.Unmarshal([]byte(historyJSON), &history); err == nil {
			messages = append(messages, history...)
		}
	}

	messages = append(messages, ollama.ChatMessage{Role: "user", Content: prompt})

	chatReq := ollama.ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	// Get session info for analytics
	company := middleware.GetCompany(r)
	sessionID := ""
	if c, err := r.Cookie("cv_sid"); err == nil {
		sessionID = c.Value
	}

	streamID := newStreamID()
	pendingStreams.Lock()
	pendingStreams.m[streamID] = pendingStream{
		Request:   chatReq,
		Company:   company,
		SessionID: sessionID,
	}
	pendingStreams.Unlock()

	// Check if model is already loaded in GPU
	modelLoaded := ollama.IsModelLoaded(model)

	// Return HTML fragment with SSE connection + user message bubble
	w.Header().Set("Content-Type", "text/html")
	Templates["chat-fragment.html"].ExecuteTemplate(w, "chat-fragment", map[string]any{
		"Prompt":      prompt,
		"StreamID":    streamID,
		"ModelLoaded": modelLoaded,
	})
}

// ChatStream handles GET /api/stream?id=xxx — SSE endpoint.
func ChatStream(w http.ResponseWriter, r *http.Request) {
	streamID := r.URL.Query().Get("id")
	if streamID == "" {
		http.Error(w, "missing stream id", http.StatusBadRequest)
		return
	}

	pendingStreams.RLock()
	ps, ok := pendingStreams.m[streamID]
	pendingStreams.RUnlock()

	if !ok {
		http.Error(w, "stream not found", http.StatusNotFound)
		return
	}

	// Clean up
	pendingStreams.Lock()
	delete(pendingStreams.m, streamID)
	pendingStreams.Unlock()

	usage := ollama.StreamChat(w, r, ps.Request)

	// Log LLM usage
	if usage.OutputTokens > 0 {
		middleware.LogLLMUsage(ps.Company, ps.SessionID, usage.Model, usage.InputTokens, usage.OutputTokens)
	}
}
