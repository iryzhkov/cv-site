package handlers

import "net/http"

func Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		NotFound(w, r)
		return
	}
	Templates["home.html"].ExecuteTemplate(w, "base", nil)
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	Templates["404.html"].ExecuteTemplate(w, "base", nil)
}
