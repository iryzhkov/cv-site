package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var BaseURL = "http://192.168.70.130:8080"

// Model represents an Ollama model.
type Model struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// TagsResponse is the response from /api/tags.
type TagsResponse struct {
	Models []Model `json:"models"`
}

// ListModels returns available models from the Ollama proxy.
func ListModels() ([]Model, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(BaseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tags TagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags.Models, nil
}

// RunningModel represents a model currently loaded in memory.
type RunningModel struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	SizeVRAM   int64  `json:"size_vram"`
	Details    struct {
		ParameterSize    string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

// PSResponse is the response from /api/ps.
type PSResponse struct {
	Models []RunningModel `json:"models"`
}

// ListRunningModels returns models currently loaded in memory.
func ListRunningModels() ([]RunningModel, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(BaseURL + "/api/ps")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ps PSResponse
	if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
		return nil, err
	}
	return ps.Models, nil
}

// IsModelLoaded checks if a specific model is currently loaded.
func IsModelLoaded(name string) bool {
	models, err := ListRunningModels()
	if err != nil {
		return false
	}
	for _, m := range models {
		if m.Name == name {
			return true
		}
	}
	return false
}

// GenerateRequest is the request body for /api/generate.
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

// ChatMessage represents a single message in a chat.
type ChatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// ChatRequest is the request body for /api/chat.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// StreamChunk is a single chunk from a streaming response.
type StreamChunk struct {
	Model    string      `json:"model"`
	Message  ChatMessage `json:"message"`
	Done     bool        `json:"done"`
	TotalDuration   int64 `json:"total_duration,omitempty"`
	EvalCount       int   `json:"eval_count,omitempty"`
	EvalDuration    int64 `json:"eval_duration,omitempty"`
	PromptEvalCount int   `json:"prompt_eval_count,omitempty"`
}

// EmbedRequest is the request body for /api/embed.
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse is the response from /api/embed.
type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// Embed generates embeddings for the given text.
func Embed(model, text string) ([]float64, error) {
	reqBody := EmbedRequest{Model: model, Input: text}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Post(BaseURL+"/api/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed failed: %s", string(b))
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}
	if len(embedResp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embedResp.Embeddings[0], nil
}

// GPUStatus represents the current state of the GPU backend.
// CPU is always assumed available via the proxy.
type GPUStatus struct {
	GPUOnline bool
}

// CheckStatus checks if the GPU backend is reachable.
// CPU fallback is always available via the proxy, so we only check GPU.
func CheckStatus() GPUStatus {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://192.168.90.212:11434/api/tags")
	if err != nil {
		return GPUStatus{}
	}
	resp.Body.Close()
	return GPUStatus{GPUOnline: true}
}
