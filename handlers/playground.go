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
	preselect := r.URL.Query().Get("model")
	if preselect == "" {
		preselect = preferredModel
	}

	Templates["playground.html"].ExecuteTemplate(w, "base", map[string]any{
		"Presets":        systemPresets,
		"Active":         "playground",
		"HasLive":        middleware.HasLiveAccess(r),
		"PreselectedModel": preselect,
	})
}

const preferredModel = "gemma3:12b"

// ModelsAPI returns model options as HTML for async loading.
// External users without token only see the restricted model.
func ModelsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	preselect := r.URL.Query().Get("preselect")
	if preselect == "" {
		preselect = preferredModel
	}

	// Restricted: only show the small model
	if !middleware.HasLiveAccess(r) {
		w.Write([]byte(`<option value="` + RestrictedModel + `">` + RestrictedModel + `</option>`))
		return
	}

	models, err := ollama.ListModels()
	if err != nil || len(models) == 0 {
		w.Write([]byte(`<option value="` + preselect + `">` + preselect + `</option>`))
		return
	}

	// Write preselected model first if it exists
	hasPreselect := false
	for _, m := range models {
		if m.Name == preselect {
			hasPreselect = true
			break
		}
	}
	if hasPreselect {
		w.Write([]byte(`<option value="` + preselect + `" selected>` + preselect + `</option>`))
	}
	for _, m := range models {
		if m.Name == preselect {
			continue
		}
		w.Write([]byte(`<option value="` + m.Name + `">` + m.Name + `</option>`))
	}
}
