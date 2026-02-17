package main

import (
    "net/http"
    "os"
)

func getOrphanedFiles(uploadDir string) (OrphanData, error) {
	diskFiles, err := getFilesOnDisk(uploadDir)
	if err != nil {
		return OrphanData{}, err
	}
	dbFiles, err := getFilesInDB()
	if err != nil {
		return OrphanData{}, err
	}

	// Build a set of disk files for reverse lookup
	diskFileSet := make(map[string]bool, len(diskFiles))
	for _, f := range diskFiles {
		diskFileSet[f] = true
	}

	var orphans []string
	for _, f := range diskFiles {
		if !dbFiles[f] {
			orphans = append(orphans, f)
		}
	}

	var reverseOrphans []string
	for f := range dbFiles {
		if !diskFileSet[f] {
			reverseOrphans = append(reverseOrphans, f)
		}
	}

	return OrphanData{
		Orphans:        orphans,
		ReverseOrphans: reverseOrphans,
	}, nil
}

func orphansHandler(w http.ResponseWriter, r *http.Request) {
	orphanData, err := getOrphanedFiles(config.UploadDir)
	if err != nil {
		renderError(w, "Error reading orphaned files", http.StatusInternalServerError)
		return
	}
	pageData := buildPageData("Orphaned Files", orphanData)
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