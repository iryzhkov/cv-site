package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type Project struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	Diagram     string   `json:"diagram,omitempty"`
	Tags        []string `json:"tags"`
	GitHub      string   `json:"github,omitempty"`
	Link        string   `json:"link,omitempty"`
	Features    []string `json:"features,omitempty"`
}

var projects []Project

func LoadProjects() error {
	data, err := os.ReadFile("data/projects.json")
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &projects)
}

func ProjectsIndex(w http.ResponseWriter, r *http.Request) {
	Templates["projects.html"].ExecuteTemplate(w, "base", map[string]any{
		"Projects": projects,
		"Active":   "projects",
	})
}

func ProjectDetail(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/projects/")
	slug = strings.TrimSuffix(slug, "/")

	if slug == "" {
		ProjectsIndex(w, r)
		return
	}

	for _, p := range projects {
		if p.Slug == slug {
			Templates["project.html"].ExecuteTemplate(w, "base", map[string]any{
				"Project": p,
				"Active":  "projects",
			})
			return
		}
	}

	NotFound(w, r)
}
