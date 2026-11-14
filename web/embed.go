// Package web serves the browser application from the same origin as the API.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var files embed.FS

func Handler() http.Handler {
	assets, _ := fs.Sub(files, "static")
	server := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hash-based application routes need no catch-all that could mask API 404s.
		name := r.URL.Path
		if name == "/" {
			name = "/index.html"
		}
		info, err := fs.Stat(assets, name[1:])
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob: data:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		server.ServeHTTP(w, r)
	})
}
