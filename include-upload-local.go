package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func localFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}

	rawPath := strings.TrimSpace(r.FormValue("filepath"))
	if rawPath == "" {
		renderError(w, "No file path provided", http.StatusBadRequest)
		return
	}

	// Resolve to an absolute, cleaned path to prevent traversal tricks
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		renderError(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Confirm the file actually exists and is a regular file (not a dir/symlink)
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			renderError(w, fmt.Sprintf("File not found: %s", absPath), http.StatusBadRequest)
		} else {
			renderError(w, fmt.Sprintf("Cannot access file: %v", err), http.StatusInternalServerError)
		}
		return
	}
	if !info.Mode().IsRegular() {
		renderError(w, "Path must point to a regular file", http.StatusBadRequest)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		renderError(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	deleteSource := r.FormValue("delete_source") == "on"

	id, warningMsg, err := processUpload(f, filepath.Base(absPath))
	if err != nil {
		renderError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if deleteSource {
		f.Close()
		if removeErr := os.Remove(absPath); removeErr != nil {
			warningMsg = strings.TrimPrefix(fmt.Sprintf("%s; could not delete source file: %v", warningMsg, removeErr), "; ")
		}
	}

	redirectWithWarning(w, r, fmt.Sprintf("/file/%d", id), warningMsg)
}
