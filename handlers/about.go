package handlers

import "net/http"

func About(w http.ResponseWriter, r *http.Request) {
	Templates["about.html"].ExecuteTemplate(w, "base", map[string]any{
		"Active": "about",
	})
}
