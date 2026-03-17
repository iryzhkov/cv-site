package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/iryzhkov/cv-site/middleware"
	"github.com/iryzhkov/cv-site/ollama"
)

// DiscordWebhookURL is set from config.
var DiscordWebhookURL = ""

var contactFile = "data/contacts.jsonl"
var contactMu sync.Mutex

type ContactMessage struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Company   string `json:"company,omitempty"`
	Timestamp string `json:"ts"`
	IsSpam    bool   `json:"spam"`
	Notified  bool   `json:"notified"`
}

func Contact(w http.ResponseWriter, r *http.Request) {
	Templates["contact.html"].ExecuteTemplate(w, "base", map[string]any{
		"Active": "contact",
	})
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

	company := middleware.GetCompany(r)

	msg := ContactMessage{
		Name:      name,
		Email:     email,
		Message:   message,
		Company:   company,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Spam check + Discord notification in background
	go processContact(msg)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="term-section">
<div class="term-line" style="color:var(--term-success)">[OK] Message sent successfully.</div>
<div class="term-line dim">I'll get back to you soon.</div>
</div>`))
}

func processContact(msg ContactMessage) {
	// Spam check via Ollama
	msg.IsSpam = checkSpam(msg)

	// Save to file
	saveContact(msg)

	// Send to Discord if not spam
	if !msg.IsSpam && DiscordWebhookURL != "" {
		msg.Notified = sendDiscord(msg)
		// Re-save with notified status
		// (not critical — the initial save already has the message)
	}
}

func checkSpam(msg ContactMessage) bool {
	prompt := fmt.Sprintf("Name: %s\nEmail: %s\nMessage: %s", msg.Name, msg.Email, msg.Message)
	system := `You are a spam detector. Respond with ONLY "spam" or "not_spam".
Spam: sales pitches, SEO services, link building, crypto, gambling, nonsense, automated messages.
Not spam: job inquiries, technical questions, collaboration, personal messages.`

	chatReq := ollama.ChatRequest{
		Model: "qwen3:8b",
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	body, _ := json.Marshal(chatReq)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(ollama.BaseURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Spam check failed: %v", err)
		return false // If check fails, assume not spam
	}
	defer resp.Body.Close()

	var result ollama.StreamChunk
	json.NewDecoder(resp.Body).Decode(&result)

	response := strings.ToLower(strings.TrimSpace(result.Message.Content))
	return strings.Contains(response, "spam") && !strings.Contains(response, "not_spam")
}

func sendDiscord(msg ContactMessage) bool {
	content := fmt.Sprintf("**New Contact from CV Site**\n\n**Name:** %s\n**Email:** %s\n**Company:** %s\n\n**Message:**\n%s",
		msg.Name, msg.Email, orDefault(msg.Company, "N/A"), msg.Message)

	payload, _ := json.Marshal(map[string]string{"content": content})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(DiscordWebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("Discord notification failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func saveContact(msg ContactMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')

	contactMu.Lock()
	defer contactMu.Unlock()

	f, err := os.OpenFile(contactFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
}

// ReadContacts returns all contact messages.
func ReadContacts() []ContactMessage {
	data, err := os.ReadFile(contactFile)
	if err != nil {
		return nil
	}

	var messages []ContactMessage
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var msg ContactMessage
		if err := dec.Decode(&msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages
}
