package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
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

	if len(parts) >= 5 && parts[3] == "tag" && parts[4] == "delete" {
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
		log.Printf("Error: fileDeleteHandler: file not found for id=%s: %v", fileID, err)
		renderError(w, "File not found", http.StatusNotFound)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error: fileDeleteHandler: failed to start transaction for file id=%s: %v", fileID, err)
		renderError(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if _, err = tx.Exec("DELETE FROM file_tags WHERE file_id=?", fileID); err != nil {
		log.Printf("Error: fileDeleteHandler: failed to delete file_tags for file id=%s: %v", fileID, err)
		renderError(w, "Failed to delete file tags", http.StatusInternalServerError)
		return
	}

	if _, err = tx.Exec("DELETE FROM file_properties WHERE file_id=?", fileID); err != nil {
		log.Printf("Error: fileDeleteHandler: failed to delete file_properties for file id=%s: %v", fileID, err)
		renderError(w, "Failed to delete file properties", http.StatusInternalServerError)
		return
	}

	if _, err = tx.Exec("DELETE FROM files WHERE id=?", fileID); err != nil {
		log.Printf("Error: fileDeleteHandler: failed to delete files record for file id=%s: %v", fileID, err)
		renderError(w, "Failed to delete file record", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("Error: fileDeleteHandler: failed to commit transaction for file id=%s: %v", fileID, err)
		renderError(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	absPath := filepath.Join(config.UploadDir, currentFile.Path)
	if err = os.Remove(absPath); err != nil {
		log.Printf("Warning: fileDeleteHandler: failed to delete physical file %s: %v", absPath, err)
	}

	// Delete thumbnail if it exists
	thumbPath := filepath.Join(config.UploadDir, "thumbnails", currentFile.Filename+".jpg")
	if _, err := os.Stat(thumbPath); err == nil {
		if err := os.Remove(thumbPath); err != nil {
			log.Printf("Warning: fileDeleteHandler: failed to delete thumbnail %s: %v", thumbPath, err)
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

	var currentFilename, currentRelPath string
	err := db.QueryRow("SELECT filename, path FROM files WHERE id=?", fileID).Scan(&currentFilename, &currentRelPath)
	if err != nil {
		log.Printf("Error: fileRenameHandler: file not found for id=%s: %v", fileID, err)
		renderError(w, "File not found", http.StatusNotFound)
		return
	}

	if currentFilename == newFilename {
		http.Redirect(w, r, "/file/"+fileID, http.StatusSeeOther)
		return
	}

	currentAbsPath := filepath.Join(config.UploadDir, currentRelPath)
	newPath := filepath.Join(config.UploadDir, newFilename)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		renderError(w, "A file with that name already exists", http.StatusConflict)
		return
	}

	if err := os.Rename(currentAbsPath, newPath); err != nil {
		log.Printf("Error: fileRenameHandler: failed to rename %s -> %s: %v", currentAbsPath, newPath, err)
		renderError(w, "Failed to rename physical file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	thumbOld := filepath.Join(config.UploadDir, "thumbnails", currentFilename+".jpg")
	thumbNew := filepath.Join(config.UploadDir, "thumbnails", newFilename+".jpg")

	if _, err := os.Stat(thumbOld); err == nil {
		if err := os.Rename(thumbOld, thumbNew); err != nil {
			log.Printf("Error: fileRenameHandler: failed to rename thumbnail %s -> %s: %v", thumbOld, thumbNew, err)
			if renameErr := os.Rename(newPath, currentAbsPath); renameErr != nil {
				log.Printf("Error: fileRenameHandler: failed to roll back file rename %s -> %s: %v", newPath, currentAbsPath, renameErr)
			}
			renderError(w, "Failed to rename thumbnail: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	newRelPath, err := filepath.Rel(config.UploadDir, newPath)
	if err != nil {
		log.Printf("Error: fileRenameHandler: failed to compute relative path for %s: %v", newPath, err)
		newRelPath = newFilename
	}

	_, err = db.Exec("UPDATE files SET filename=?, path=? WHERE id=?", newFilename, newRelPath, fileID)
	if err != nil {
		log.Printf("Error: fileRenameHandler: failed to update database for file id=%s: %v", fileID, err)
		if renameErr := os.Rename(newPath, currentAbsPath); renameErr != nil {
			log.Printf("Error: fileRenameHandler: failed to roll back file rename %s -> %s: %v", newPath, currentAbsPath, renameErr)
		}
		if _, statErr := os.Stat(thumbNew); statErr == nil {
			if renameErr := os.Rename(thumbNew, thumbOld); renameErr != nil {
				log.Printf("Error: fileRenameHandler: failed to roll back thumbnail rename %s -> %s: %v", thumbNew, thumbOld, renameErr)
			}
		}
		renderError(w, "Failed to update database", http.StatusInternalServerError)
		return
	}

	// Recompute properties in case the extension changed
	if _, err := db.Exec("DELETE FROM file_properties WHERE file_id = ?", fileID); err != nil {
		log.Printf("Warning: fileRenameHandler: failed to delete old properties for file id=%s: %v", fileID, err)
	}
	if id, err := strconv.ParseInt(fileID, 10, 64); err == nil {
		computeProperties(id, newPath)
	} else {
		log.Printf("Warning: fileRenameHandler: failed to parse file id %s for property recompute: %v", fileID, err)
	}

	http.Redirect(w, r, "/file/"+fileID, http.StatusSeeOther)
}

func checkFileConflictStrict(filename string) (string, string, int64, error) {
	finalPath := filepath.Join(config.UploadDir, filename)
	if _, err := os.Stat(finalPath); err == nil {
		existingID, dbErr := getFileIDByName(filename)
		if dbErr != nil {
			log.Printf("Warning: checkFileConflictStrict: file %s exists on disk but not in DB: %v", filename, dbErr)
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