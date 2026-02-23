package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
)

func fileRouter(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")

	if len(parts) >= 4 && parts[3] == "delete" {
		fileDeleteHandler(w, r, parts)
		return
	}

	if len(parts) >= 4 && parts[3] == "rename" {
		fileRenameHandler(w, r, parts)
		return
	}

	if len(parts) >= 7 && parts[3] == "tag" {
		tagActionHandler(w, r, parts)
		return
	}

	fileHandler(w, r)
}

func fileDeleteHandler(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/file/"+parts[2], http.StatusSeeOther)
		return
	}

	fileID := parts[2]

	var currentFile File
	err := db.QueryRow("SELECT id, filename, path FROM files WHERE id=?", fileID).Scan(&currentFile.ID, &currentFile.Filename, &currentFile.Path)
	if err != nil {
		renderError(w, "File not found", http.StatusNotFound)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		renderError(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if _, err = tx.Exec("DELETE FROM file_tags WHERE file_id=?", fileID); err != nil {
		renderError(w, "Failed to delete file tags", http.StatusInternalServerError)
		return
	}

	if _, err = tx.Exec("DELETE FROM files WHERE id=?", fileID); err != nil {
		renderError(w, "Failed to delete file record", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		renderError(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	if err = os.Remove(currentFile.Path); err != nil {
		log.Printf("Warning: Failed to delete physical file %s: %v", currentFile.Path, err)
	}

	// Delete thumbnail if it exists
	thumbPath := filepath.Join(config.UploadDir, "thumbnails", currentFile.Filename+".jpg")
	if _, err := os.Stat(thumbPath); err == nil {
		if err := os.Remove(thumbPath); err != nil {
			log.Printf("Warning: Failed to delete thumbnail %s: %v", thumbPath, err)
		}
	}

	http.Redirect(w, r, "/?deleted="+currentFile.Filename, http.StatusSeeOther)
}

func fileRenameHandler(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/file/"+parts[2], http.StatusSeeOther)
		return
	}

	fileID := parts[2]
	newFilename := sanitizeFilename(strings.TrimSpace(r.FormValue("newfilename")))

	if newFilename == "" {
		renderError(w, "New filename cannot be empty", http.StatusBadRequest)
		return
	}

	var currentFilename, currentPath string
	err := db.QueryRow("SELECT filename, path FROM files WHERE id=?", fileID).Scan(&currentFilename, &currentPath)
	if err != nil {
		renderError(w, "File not found", http.StatusNotFound)
		return
	}

	if currentFilename == newFilename {
		http.Redirect(w, r, "/file/"+fileID, http.StatusSeeOther)
		return
	}

	newPath := filepath.Join(config.UploadDir, newFilename)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		renderError(w, "A file with that name already exists", http.StatusConflict)
		return
	}

	if err := os.Rename(currentPath, newPath); err != nil {
		renderError(w, "Failed to rename physical file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	thumbOld := filepath.Join(config.UploadDir, "thumbnails", currentFilename+".jpg")
	thumbNew := filepath.Join(config.UploadDir, "thumbnails", newFilename+".jpg")

	if _, err := os.Stat(thumbOld); err == nil {
		if err := os.Rename(thumbOld, thumbNew); err != nil {
			os.Rename(newPath, currentPath)
			renderError(w, "Failed to rename thumbnail: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	_, err = db.Exec("UPDATE files SET filename=?, path=? WHERE id=?", newFilename, newPath, fileID)
	if err != nil {
		os.Rename(newPath, currentPath)
		if _, err := os.Stat(thumbNew); err == nil {
			os.Rename(thumbNew, thumbOld)
		}
		renderError(w, "Failed to update database", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/file/"+fileID, http.StatusSeeOther)
}

func checkFileConflictStrict(filename string) (string, string, int64, error) {
	finalPath := filepath.Join(config.UploadDir, filename)
	if _, err := os.Stat(finalPath); err == nil {
		existingID, dbErr := getFileIDByName(filename)
		if dbErr != nil {
			return "", "", 0, fmt.Errorf("a file with that name already exists")
		}
		return "", "", existingID, nil
	} else if !os.IsNotExist(err) {
		return "", "", 0, fmt.Errorf("failed to check for existing file: %v", err)
	}
	return filename, finalPath, 0, nil
}

func getFileIDByName(filename string) (int64, error) {
	var id int64
	err := db.QueryRow("SELECT id FROM files WHERE filename = ?", filename).Scan(&id)
	return id, err
}