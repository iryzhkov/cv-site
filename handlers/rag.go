package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/iryzhkov/cv-site/ollama"
)

// ragSession stores an uploaded document split into chunks with embeddings.
type ragSession struct {
	Chunks     []string
	Embeddings [][]float64
}

var ragSessions = struct {
	sync.RWMutex
	m map[string]*ragSession
}{m: make(map[string]*ragSession)}

func RAG(w http.ResponseWriter, r *http.Request) {
	Templates["rag.html"].ExecuteTemplate(w, "base", map[string]any{
		"Active": "rag",
	})
}

// RAGIngest handles POST /api/rag/ingest — splits document into chunks and embeds them.
func RAGIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	document := r.FormValue("document")
	if document == "" {
		http.Error(w, "document required", http.StatusBadRequest)
		return
	}

	// Split into chunks (~200 words each, overlap 50)
	chunks := chunkText(document, 200, 50)
	if len(chunks) == 0 {
		http.Error(w, "document too short", http.StatusBadRequest)
		return
	}

	// Embed each chunk
	embeddings := make([][]float64, 0, len(chunks))
	for _, chunk := range chunks {
		emb, err := ollama.Embed("nomic-embed-text", chunk)
		if err != nil {
			// Try a fallback model
			emb, err = ollama.Embed("all-minilm", chunk)
			if err != nil {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(`<div class="text-red-400 text-sm">Error embedding document. Make sure an embedding model is available.</div>`))
				return
			}
		}
		embeddings = append(embeddings, emb)
	}

	// Store session
	id := newRAGSessionID()
	ragSessions.Lock()
	ragSessions.m[id] = &ragSession{Chunks: chunks, Embeddings: embeddings}
	ragSessions.Unlock()

	w.Header().Set("Content-Type", "text/html")
	Templates["rag-ready.html"].ExecuteTemplate(w, "rag-ready", map[string]any{
		"SessionID":  id,
		"ChunkCount": len(chunks),
	})
}

// RAGQuery handles POST /api/rag/query — finds relevant chunks and streams an answer.
func RAGQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	query := r.FormValue("query")
	model := r.FormValue("model")

	if sessionID == "" || query == "" {
		http.Error(w, "session_id and query required", http.StatusBadRequest)
		return
	}
	if model == "" {
		model = "qwen3:4b"
	}

	ragSessions.RLock()
	session, ok := ragSessions.m[sessionID]
	ragSessions.RUnlock()

	if !ok {
		http.Error(w, "session not found — please re-upload document", http.StatusNotFound)
		return
	}

	// Embed the query
	queryEmb, err := ollama.Embed("nomic-embed-text", query)
	if err != nil {
		queryEmb, err = ollama.Embed("all-minilm", query)
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="text-red-400 text-sm">Error embedding query</div>`))
			return
		}
	}

	// Find top 3 most relevant chunks
	type scored struct {
		Index int
		Score float64
	}
	scores := make([]scored, len(session.Embeddings))
	for i, emb := range session.Embeddings {
		scores[i] = scored{Index: i, Score: cosineSim(queryEmb, emb)}
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })

	topK := 3
	if len(scores) < topK {
		topK = len(scores)
	}

	var contextChunks []string
	var chunkInfos []map[string]any
	for i := 0; i < topK; i++ {
		idx := scores[i].Index
		contextChunks = append(contextChunks, session.Chunks[idx])
		chunkInfos = append(chunkInfos, map[string]any{
			"Index": idx + 1,
			"Score": scores[i].Score,
			"Text":  truncate(session.Chunks[idx], 150),
		})
	}

	// Build the prompt with context
	context := strings.Join(contextChunks, "\n\n---\n\n")
	systemPrompt := "You are a helpful assistant. Answer the user's question based ONLY on the provided context. If the context doesn't contain the answer, say so.\n\nContext:\n" + context

	// Store as a pending stream
	streamID := newStreamID()
	chatReq := ollama.ChatRequest{
		Model: model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: query},
		},
		Stream: true,
	}

	pendingStreams.Lock()
	pendingStreams.m[streamID] = pendingStream{Request: chatReq}
	pendingStreams.Unlock()

	w.Header().Set("Content-Type", "text/html")
	Templates["rag-response.html"].ExecuteTemplate(w, "rag-response", map[string]any{
		"Query":    query,
		"Chunks":   chunkInfos,
		"StreamID": streamID,
	})
}

func chunkText(text string, wordsPerChunk, overlap int) []string {
	words := strings.Fields(text)
	var chunks []string
	for i := 0; i < len(words); i += wordsPerChunk - overlap {
		end := i + wordsPerChunk
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		if len(chunk) > 20 {
			chunks = append(chunks, chunk)
		}
		if end == len(words) {
			break
		}
	}
	return chunks
}

func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func newRAGSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
