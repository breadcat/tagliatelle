package app

import (
	"log"
	"net/http"
	"os"
)

type OrphanFile struct {
	ID       int
	Filename string
}

type OrphanData struct {
	Orphans        []OrphanFile // on disk, not in DB
	ReverseOrphans []OrphanFile // in DB, not on disk
}

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

	var orphans []OrphanFile
	for _, f := range diskFiles {
		if _, ok := dbFiles[f]; !ok {
			orphans = append(orphans, OrphanFile{
				Filename: f,
			})
		}
	}

	var reverseOrphans []OrphanFile
	for filename, file := range dbFiles {
		if !diskFileSet[filename] {
			reverseOrphans = append(reverseOrphans, OrphanFile{
				ID:       file.ID,
				Filename: file.Filename,
			})
		}
	}

	return OrphanData{
		Orphans:        orphans,
		ReverseOrphans: reverseOrphans,
	}, nil
}

func orphansHandler(w http.ResponseWriter, r *http.Request) {
	orphanData, err := getOrphanedFiles(Cfg.UploadDir)
	if err != nil {
		log.Printf("Error: orphansHandler: failed to read orphaned files: %v", err)
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
