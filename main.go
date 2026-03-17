package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/iryzhkov/cv-site/handlers"
	"github.com/iryzhkov/cv-site/middleware"
	"github.com/iryzhkov/cv-site/ollama"
)

func main() {
	cfg := LoadConfig()
	ollama.BaseURL = cfg.OllamaURL

	// Load tokens
	middleware.LoadTokens()

	// Load projects
	if err := handlers.LoadProjects(); err != nil {
		log.Printf("Warning: failed to load projects: %v", err)
	}

	// Parse templates — each page gets its own template set (base + page + partials)
	partials := []string{
		"templates/partials/gpu-status.html",
		"templates/partials/loading.html",
	}
	pages := []string{
		"home.html", "about.html", "404.html",
		"playground.html", "projects.html", "project.html",
		"benchmarks.html", "rag.html", "vision.html", "admin.html",
	}
	funcMap := template.FuncMap{
		"raw": func(s string) template.HTML { return template.HTML(s) },
	}
	handlers.Templates = make(map[string]*template.Template)
	for _, page := range pages {
		files := append([]string{"templates/base.html", "templates/" + page}, partials...)
		t, err := template.New("").Funcs(funcMap).ParseFiles(files...)
		if err != nil {
			log.Fatalf("Failed to parse template %s: %v", page, err)
		}
		handlers.Templates[page] = t
	}

	// Fragment templates (no base layout)
	fragments := []string{
		"chat-fragment.html", "benchmark-result.html",
		"rag-ready.html", "rag-response.html", "vision-response.html",
	}
	for _, frag := range fragments {
		files := append([]string{"templates/" + frag}, partials...)
		t, err := template.New("").Funcs(funcMap).ParseFiles(files...)
		if err != nil {
			log.Fatalf("Failed to parse fragment %s: %v", frag, err)
		}
		handlers.Templates[frag] = t
	}

	// GPU status partial (standalone)
	t, err := template.New("").Funcs(funcMap).ParseFiles(partials...)
	if err != nil {
		log.Fatalf("Failed to parse gpu-status partial: %v", err)
	}
	handlers.Templates["gpu-status.html"] = t

	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("/", handlers.Home)
	mux.HandleFunc("/about", handlers.About)
	mux.HandleFunc("/projects", handlers.ProjectsIndex)
	mux.HandleFunc("/projects/", handlers.ProjectDetail)
	mux.HandleFunc("/playground", handlers.Playground)
	mux.HandleFunc("/benchmarks", handlers.Benchmarks)
	mux.HandleFunc("/rag", handlers.RAG)
	mux.HandleFunc("/vision", handlers.Vision)

	// Admin (local only)
	mux.HandleFunc("/admin", middleware.LocalOnly(handlers.Admin))
	mux.HandleFunc("/admin/chart-data", middleware.LocalOnly(handlers.AdminChartData))
	mux.HandleFunc("/admin/tokens/create", middleware.LocalOnly(handlers.AdminCreateToken))
	mux.HandleFunc("/admin/tokens/revoke", middleware.LocalOnly(handlers.AdminRevokeToken))
	mux.HandleFunc("/admin/tokens/delete", middleware.LocalOnly(handlers.AdminDeleteToken))

	// REST API for token management (local network only)
	mux.HandleFunc("/api/admin/tokens", middleware.LocalOnly(handlers.APIListTokens))
	mux.HandleFunc("/api/admin/tokens/create", middleware.LocalOnly(handlers.APICreateToken))
	mux.HandleFunc("/api/admin/tokens/revoke", middleware.LocalOnly(handlers.APIRevokeToken))
	mux.HandleFunc("/api/admin/tokens/delete", middleware.LocalOnly(handlers.APIDeleteToken))

	// API endpoints
	mux.HandleFunc("/api/models", handlers.ModelsAPI)
	mux.HandleFunc("/api/gpu-status", handlers.GPUStatus)
	mux.HandleFunc("/api/loaded-models", handlers.LoadedModels)
	mux.HandleFunc("/api/chat", handlers.ChatSubmit)
	mux.HandleFunc("/api/stream", handlers.ChatStream)
	mux.HandleFunc("/api/benchmark", handlers.RunBenchmark)
	mux.HandleFunc("/api/rag/ingest", handlers.RAGIngest)
	mux.HandleFunc("/api/rag/query", handlers.RAGQuery)
	mux.HandleFunc("/api/vision", handlers.VisionAnalyze)

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Wrap with middleware
	handler := middleware.AccessControl(middleware.Analytics(mux))

	log.Printf("Starting cv-site on :%s (ollama: %s)", cfg.Port, cfg.OllamaURL)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
}
