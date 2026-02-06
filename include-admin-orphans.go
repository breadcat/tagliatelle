package main

import (
    "net/http"
    "os"
)

func getOrphanedFiles(uploadDir string) ([]string, error) {
	diskFiles, err := getFilesOnDisk(uploadDir)
	if err != nil {
		return nil, err
	}

	dbFiles, err := getFilesInDB()
	if err != nil {
		return nil, err
	}

	var orphans []string
	for _, f := range diskFiles {
		if !dbFiles[f] {
			orphans = append(orphans, f)
		}
	}
	return orphans, nil
}

func orphansHandler(w http.ResponseWriter, r *http.Request) {
	orphans, err := getOrphanedFiles(config.UploadDir)
	if err != nil {
		renderError(w, "Error reading orphaned files", http.StatusInternalServerError)
		return
	}

	pageData := buildPageData("Orphaned Files", orphans)
	renderTemplate(w, "orphans.html", pageData)
}

func getFilesOnDisk(uploadDir string) ([]string, error) {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}