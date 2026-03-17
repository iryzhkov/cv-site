package handlers

import (
	"net/http"

	"github.com/iryzhkov/cv-site/middleware"

	"github.com/iryzhkov/cv-site/ollama"
)

var systemPresets = []struct {
	Name   string
	Prompt string
}{
	{"None", ""},
	{"Code Review", "You are an expert code reviewer. Analyze code for bugs, performance issues, and best practices. Be concise and actionable."},
	{"Explain Simply", "You are a patient teacher. Explain concepts in simple terms using analogies. Avoid jargon unless defining it."},
	{"Technical Writer", "You are a technical writer. Write clear, structured documentation. Use headings, bullet points, and code examples."},
	{"ML Interview", "You are an ML interview coach. Ask follow-up questions, test understanding, and provide detailed explanations of ML concepts."},
}

func Playground(w http.ResponseWriter, r *http.Request) {
	Templates["playground.html"].ExecuteTemplate(w, "base", map[string]any{
		"Presets": systemPresets,
		"Active":  "playground",
		"HasLive": middleware.HasLiveAccess(r),
	})
}

const preferredModel = "gemma3:12b"

// ModelsAPI returns model options as HTML for async loading.
// Puts the preferred model first if available.
func ModelsAPI(w http.ResponseWriter, r *http.Request) {
	models, err := ollama.ListModels()
	if err != nil || len(models) == 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<option value="` + preferredModel + `">` + preferredModel + `</option>`))
		return
	}
	w.Header().Set("Content-Type", "text/html")

	// Write preferred model first if it exists
	hasPreferred := false
	for _, m := range models {
		if m.Name == preferredModel {
			hasPreferred = true
			break
		}
	}
	if hasPreferred {
		w.Write([]byte(`<option value="` + preferredModel + `">` + preferredModel + `</option>`))
	}
	for _, m := range models {
		if m.Name == preferredModel {
			continue
		}
		w.Write([]byte(`<option value="` + m.Name + `">` + m.Name + `</option>`))
	}
}
