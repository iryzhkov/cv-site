package handlers

import (
	"encoding/base64"
	"io"
	"net/http"

	"github.com/iryzhkov/cv-site/ollama"
)

func Vision(w http.ResponseWriter, r *http.Request) {
	Templates["vision.html"].ExecuteTemplate(w, "base", map[string]any{
		"Active": "vision",
	})
}

// VisionAnalyze handles POST /api/vision — accepts an image upload and streams analysis.
func VisionAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	model := r.FormValue("model")
	prompt := r.FormValue("prompt")
	if model == "" {
		model = "gemma3:4b"
	}
	if prompt == "" {
		prompt = "Describe this image in detail. What do you see?"
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	imgData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read image", http.StatusInternalServerError)
		return
	}

	// Base64 encode the image for Ollama
	b64 := base64.StdEncoding.EncodeToString(imgData)

	chatReq := ollama.ChatRequest{
		Model: model,
		Messages: []ollama.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
				Images:  []string{b64},
			},
		},
		Stream: true,
	}

	streamID := newStreamID()
	pendingStreams.Lock()
	pendingStreams.m[streamID] = pendingStream{Request: chatReq}
	pendingStreams.Unlock()

	w.Header().Set("Content-Type", "text/html")
	Templates["vision-response.html"].ExecuteTemplate(w, "vision-response", map[string]any{
		"StreamID": streamID,
		"Prompt":   prompt,
	})
}
