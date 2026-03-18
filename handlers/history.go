package handlers

import "net/http"

func History(w http.ResponseWriter, r *http.Request) {
	Templates["history.html"].ExecuteTemplate(w, "base", map[string]any{
		"Active": "history",
	})
}
