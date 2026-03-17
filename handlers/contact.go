package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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

var DiscordWebhookURL = ""
var contactFile = "data/contacts.json"
var contactMu sync.Mutex

type ContactMessage struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Company   string `json:"company,omitempty"`
	Timestamp string `json:"ts"`
	IsSpam    bool   `json:"spam"`
	Notified  bool   `json:"notified"`
	Read      bool   `json:"read"`
	Starred   bool   `json:"starred"`
}

func loadContacts() []ContactMessage {
	contactMu.Lock()
	defer contactMu.Unlock()
	data, err := os.ReadFile(contactFile)
	if err != nil {
		return nil
	}
	var msgs []ContactMessage
	json.Unmarshal(data, &msgs)
	return msgs
}

func saveContacts(msgs []ContactMessage) {
	contactMu.Lock()
	defer contactMu.Unlock()
	data, _ := json.MarshalIndent(msgs, "", "  ")
	os.WriteFile(contactFile, data, 0644)
}

func newContactID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ReadContacts returns all contacts (exported for admin).
func ReadContacts() []ContactMessage {
	return loadContacts()
}

func UnreadCount() int {
	count := 0
	for _, m := range loadContacts() {
		if !m.Read && !m.IsSpam {
			count++
		}
	}
	return count
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
		ID:        newContactID(),
		Name:      name,
		Email:     email,
		Message:   message,
		Company:   company,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	go processContact(msg)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="term-section">
<div class="term-line" style="color:var(--term-success)">[OK] Message sent successfully.</div>
<div class="term-line dim">I'll get back to you soon.</div>
</div>`))
}

func processContact(msg ContactMessage) {
	msg.IsSpam = checkSpam(msg)

	if !msg.IsSpam && DiscordWebhookURL != "" {
		msg.Notified = sendDiscord(msg)
		if msg.Notified {
			msg.Read = true // Auto-mark as read if sent to Discord
		}
	}

	msgs := loadContacts()
	msgs = append(msgs, msg)
	saveContacts(msgs)
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
		return false
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

// --- Admin API for contact management ---

func AdminMessages(w http.ResponseWriter, r *http.Request) {
	Templates["messages.html"].ExecuteTemplate(w, "base", map[string]any{
		"Messages": loadContacts(),
		"Active":   "admin",
	})
}

func AdminMessageAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := r.FormValue("action")
	ids := strings.Split(r.FormValue("ids"), ",")

	msgs := loadContacts()

	switch action {
	case "read":
		for i := range msgs {
			for _, id := range ids {
				if msgs[i].ID == id {
					msgs[i].Read = true
				}
			}
		}
	case "unread":
		for i := range msgs {
			for _, id := range ids {
				if msgs[i].ID == id {
					msgs[i].Read = false
				}
			}
		}
	case "star":
		for i := range msgs {
			for _, id := range ids {
				if msgs[i].ID == id {
					msgs[i].Starred = true
				}
			}
		}
	case "unstar":
		for i := range msgs {
			for _, id := range ids {
				if msgs[i].ID == id {
					msgs[i].Starred = false
				}
			}
		}
	case "delete":
		filtered := msgs[:0]
		deleteSet := make(map[string]bool)
		for _, id := range ids {
			deleteSet[id] = true
		}
		for _, m := range msgs {
			if !deleteSet[m.ID] {
				filtered = append(filtered, m)
			}
		}
		msgs = filtered
	}

	saveContacts(msgs)

	// If fetch request, return JSON; otherwise redirect
	if r.Header.Get("Accept") == "application/json" || r.Header.Get("X-Requested-With") != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	} else {
		http.Redirect(w, r, "/admin/messages", http.StatusSeeOther)
	}
}
