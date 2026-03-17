package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// AnalyticsEvent is a single tracked event.
type AnalyticsEvent struct {
	Timestamp    string `json:"ts"`
	Path         string `json:"path"`
	Method       string `json:"method"`
	Company      string `json:"company,omitempty"`
	IP           string `json:"ip"`
	UserAgent    string `json:"ua"`
	Referrer     string `json:"ref,omitempty"`
	IsLocal      bool   `json:"local"`
	SessionID    string `json:"sid,omitempty"`
	// LLM usage fields (only for inference events)
	Model        string `json:"model,omitempty"`
	InputTokens  int    `json:"in_tok,omitempty"`
	OutputTokens int    `json:"out_tok,omitempty"`
}

// LogLLMUsage writes an LLM usage event.
func LogLLMUsage(company, sessionID, model string, inputTokens, outputTokens int) {
	evt := AnalyticsEvent{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Path:         "/api/chat",
		Method:       "LLM",
		Company:      company,
		SessionID:    sessionID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
	go writeEvent(evt)
}

var analyticsFile = "data/analytics.jsonl"
var analyticsMu sync.Mutex

// Analytics middleware logs requests to JSONL.
func Analytics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip static files, API polling, and HTMX partial requests
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/gpu-status" || r.URL.Path == "/api/loaded-models" || r.URL.Path == "/api/models" {
			next.ServeHTTP(w, r)
			return
		}

		// Session tracking via cookie
		sessionID := ""
		if c, err := r.Cookie("cv_sid"); err == nil {
			sessionID = c.Value
		} else {
			b := make([]byte, 8)
			rand.Read(b)
			sessionID = hex.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{
				Name:     "cv_sid",
				Value:    sessionID,
				Path:     "/",
				MaxAge:   86400,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}

		evt := AnalyticsEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Path:      r.URL.Path,
			Method:    r.Method,
			Company:   GetCompany(r),
			IP:        getClientIP(r),
			UserAgent: r.UserAgent(),
			Referrer:  r.Referer(),
			IsLocal:   IsLocal(r),
			SessionID: sessionID,
		}

		go writeEvent(evt)

		next.ServeHTTP(w, r)
	})
}

func writeEvent(evt AnalyticsEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	data = append(data, '\n')

	analyticsMu.Lock()
	defer analyticsMu.Unlock()

	f, err := os.OpenFile(analyticsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
}

// ReadAnalytics reads all analytics events from the JSONL file.
func ReadAnalytics() []AnalyticsEvent {
	data, err := os.ReadFile(analyticsFile)
	if err != nil {
		return nil
	}

	var events []AnalyticsEvent
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var evt AnalyticsEvent
		if err := dec.Decode(&evt); err != nil {
			continue
		}
		events = append(events, evt)
	}
	return events
}
