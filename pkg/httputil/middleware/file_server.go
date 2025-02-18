package middleware

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"log"
)

// Static returns an http.Handler that serves static files.
// If the embeddedFS arg is not nil, it uses serveEmbedded; else, serveLocal
//
// Example usage:
//
//	mux := http.NewServeMux()
//	mux.Handle("GET /", middleware.Static("dist", true, embeddedFS))
//
// This will serve files from the "dist" directory of the embedded file system and use "index.html"
// as a fallback for routes not directly mapped to a file.
func Static(directory string, spaFallback bool, embeddedFS *embed.FS) http.Handler {
	if embeddedFS != nil {
		return serveEmbedded(*embeddedFS, directory, spaFallback)
	}
	return serveLocal(directory, spaFallback)
}

func serveEmbedded(embeddedFS embed.FS, baseDir string, spaFallback bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(baseDir, r.URL.Path)
		if !isValidFilePath(filePath) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		fileData, fileInfo, err := readFileFromFS(embeddedFS, filePath)
		if err != nil && spaFallback {
			filePath = filepath.Join(baseDir, "index.html")
			fileData, fileInfo, err = readFileFromFS(embeddedFS, filePath)
		}

		if err != nil {
			http.NotFound(w, r)
			return
		}

		setContentType(w, filePath)
		http.ServeContent(w, r, filePath, fileInfo.ModTime(), bytes.NewReader(fileData))
	})
}

// serveLocal returns an http.Handler that serves static files from the given directory in the host machine/container
func serveLocal(directory string, spaFallback bool) http.Handler {
	absDir, err := filepath.Abs(directory)
	if err != nil {
		log.Fatalf("Failed to resolve absolute path for %s: %v", directory, err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		cleanPath := filepath.Clean(path)

		requestedPath := filepath.Join(absDir, cleanPath)
		if !isSubPath(absDir, requestedPath) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		fileInfo, err := os.Stat(requestedPath)
		if err != nil && spaFallback {
			indexPath := filepath.Join(absDir, "index.html")
			requestedPath = indexPath
			fileInfo, err = os.Stat(indexPath)
		}

		if err != nil {
			http.NotFound(w, r)
			return
		}

		if fileInfo.IsDir() && !spaFallback {
			http.Error(w, "Directory listing not allowed", http.StatusForbidden)
			return
		}

		setContentType(w, requestedPath)
		http.ServeFile(w, r, requestedPath)
	})
}

// isValidFilePath checks if the file path is valid and does not contain any directory traversal attempts.
func isValidFilePath(filePath string) bool {
	cleaned := filepath.Clean(filePath)
	return !strings.HasPrefix(cleaned, ".")
}

// readFileFromFS reads a file from the embedded filesystem and returns its data and FileInfo.
func readFileFromFS(embeddedFS embed.FS, filePath string) ([]byte, fs.FileInfo, error) {
	fileData, err := fs.ReadFile(embeddedFS, filePath)
	if err != nil {
		return nil, nil, err
	}

	fileInfo, err := fs.Stat(embeddedFS, filePath)
	if err != nil {
		return nil, nil, err
	}

	return fileData, fileInfo, nil
}

// isSubPath checks if a path is a subdirectory of the base directory.
func isSubPath(baseDir, path string) bool {
	rel, err := filepath.Rel(baseDir, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// setContentType sets the Content-Type header based on the file extension.
func setContentType(w http.ResponseWriter, filePath string) {
	if ext := filepath.Ext(filePath); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	}
}
