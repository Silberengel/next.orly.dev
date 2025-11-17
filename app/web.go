package app

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed web/dist
var reactAppFS embed.FS

// GetReactAppFS returns a http.FileSystem from the embedded React app
func GetReactAppFS() http.FileSystem {
	webDist, err := fs.Sub(reactAppFS, "web/dist")
	if err != nil {
		panic("Failed to load embedded web app: " + err.Error())
	}
	return http.FS(webDist)
}

// ServeEmbeddedWeb serves the embedded web application
func ServeEmbeddedWeb(w http.ResponseWriter, r *http.Request) {
	// Get the file path from the request
	path := r.URL.Path

	// If path is root, serve index.html
	if path == "/" || path == "" {
		path = "/index.html"
	}

	// Remove leading slash for filesystem access
	fsPath := strings.TrimPrefix(path, "/")

	// Try to open the file from embedded filesystem
	file, err := GetReactAppFS().Open(fsPath)
	if err != nil {
		// If file not found and it's not a root request, try index.html (for SPA routing)
		if path != "/index.html" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Get file info to determine if it's a directory
	stat, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// If it's a directory, redirect to root
	if stat.IsDir() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Set proper Content-Type based on file extension
	ext := filepath.Ext(fsPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		// Fallback for common file types
		switch ext {
		case ".js":
			contentType = "application/javascript"
		case ".css":
			contentType = "text/css"
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".json":
			contentType = "application/json"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".svg":
			contentType = "image/svg+xml"
		case ".ico":
			contentType = "image/x-icon"
		default:
			contentType = "application/octet-stream"
		}
	}

	// Set Content-Type header
	w.Header().Set("Content-Type", contentType)

	// Set cache headers for static assets (but not HTML)
	if ext != ".html" {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	}

	// Serve the file
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}
