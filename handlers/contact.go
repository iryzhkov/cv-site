package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iryzhkov/cv-site/middleware"
)

// N8NWebhookURL is the n8n webhook that receives contact messages.
var N8NWebhookURL = ""

func Contact(w http.ResponseWriter, r *http.Request) {
	Templates["contact.html"].ExecuteTemplate(w, "base", map[string]any{
		"Active": "contact",
	})
}

type contactMessage struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	IP        string `json:"ip"`
	Company   string `json:"company,omitempty"`
}

func ContactSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	message := strings.TrimSpace(r.FormValue("message"))

	if name == "" || message == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="term-line" style="color:var(--term-error)">Error: name and message are required.</div>`))
		return
	}

	if N8NWebhookURL == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="term-line" style="color:var(--term-error)">Error: contact form not configured.</div>`))
		return
	}

	// Get company from access token if present
	company := ""
	if c, err := r.Cookie("cv_token"); err == nil {
		for _, t := range middleware.GetTokens() {
			if t.Token == c.Value && t.Active {
				company = t.Company
				break
			}
		}
	}

	msg := contactMessage{
		Name:      name,
		Email:     email,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		IP:        r.RemoteAddr,
		Company:   company,
	}

	body, _ := json.Marshal(msg)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(N8NWebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`<div class="term-line" style="color:var(--term-error)">Error sending message: %s</div>`, err.Error())))
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="term-section">
<div class="term-line" style="color:var(--term-success)">[OK] Message sent successfully.</div>
<div class="term-line dim">I'll get back to you soon.</div>
</div>`))
}
