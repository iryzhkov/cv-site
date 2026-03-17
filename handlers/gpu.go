package handlers

import (
	"fmt"
	"net/http"

	"github.com/iryzhkov/cv-site/ollama"
)

func GPUStatus(w http.ResponseWriter, r *http.Request) {
	status := ollama.CheckStatus()
	Templates["gpu-status.html"].ExecuteTemplate(w, "gpu-status", status)
}

func LoadedModels(w http.ResponseWriter, r *http.Request) {
	models, err := ollama.ListRunningModels()
	w.Header().Set("Content-Type", "text/html")
	if err != nil || len(models) == 0 {
		w.Write([]byte(`<p class="text-xs text-gray-600 italic">No models loaded</p>`))
		return
	}
	for _, m := range models {
		vramGB := float64(m.SizeVRAM) / (1024 * 1024 * 1024)
		backend := "GPU"
		badgeClass := "badge-green"
		if m.SizeVRAM == 0 {
			backend = "CPU"
			badgeClass = "badge-yellow"
			vramGB = float64(m.Size) / (1024 * 1024 * 1024)
		}
		fmt.Fprintf(w,
			`<div class="flex items-center justify-between bg-surface-300/50 rounded-lg px-2.5 py-1.5 border border-white/5 mb-1.5">`+
				`<span class="text-xs text-gray-300 font-mono truncate">%s</span>`+
				`<span class="badge %s text-[10px] px-1.5 py-0.5 ml-2 flex-shrink-0">%s %.1fGB</span>`+
				`</div>`,
			m.Name, badgeClass, backend, vramGB,
		)
	}
}
