package main

import (
    "fmt"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

func generateThumbnailAtTime(videoPath, uploadDir, filename, timestamp string) error {
	thumbDir := filepath.Join(uploadDir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnails directory: %v", err)
	}

	thumbPath := filepath.Join(thumbDir, filename+".jpg")

	cmd := exec.Command("ffmpeg", "-y", "-ss", timestamp, "-i", videoPath, "-vframes", "1", "-vf", "scale=400:-1", thumbPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate thumbnail at %s: %v", timestamp, err)
	}

	return nil
}

func getVideoFiles() ([]VideoFile, error) {
	videoExts := []string{".mp4", ".webm", ".mov", ".avi", ".mkv", ".m4v"}

	rows, err := db.Query(`SELECT id, filename, path FROM files ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []VideoFile
	for rows.Next() {
		var v VideoFile
		if err := rows.Scan(&v.ID, &v.Filename, &v.Path); err != nil {
			continue
		}

		// Check if it's a video file
		isVideo := false
		ext := strings.ToLower(filepath.Ext(v.Filename))
		for _, vidExt := range videoExts {
			if ext == vidExt {
				isVideo = true
				break
			}
		}

		if !isVideo {
			continue
		}

		v.EscapedFilename = url.PathEscape(v.Filename)
		thumbPath := filepath.Join(config.UploadDir, "thumbnails", v.Filename+".jpg")
		v.ThumbnailPath = "/uploads/thumbnails/" + v.EscapedFilename + ".jpg"

		if _, err := os.Stat(thumbPath); err == nil {
			v.HasThumbnail = true
		}

		videos = append(videos, v)
	}

	return videos, nil
}



func thumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	allVideos, err := getVideoFiles()
	if err != nil {
		renderError(w, "Failed to get video files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	missing, err := getMissingThumbnailVideos()
	if err != nil {
		renderError(w, "Failed to get video files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pageData := buildPageData("Thumbnail Management", struct {
		AllVideos         []VideoFile
		MissingThumbnails []VideoFile
		Error             string
		Success           string
	}{
		AllVideos:         allVideos,
		MissingThumbnails: missing,
		Error:             r.URL.Query().Get("error"),
		Success:           r.URL.Query().Get("success"),
	})

	renderTemplate(w, "thumbnails.html", pageData)
}

func generateThumbnailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	action := r.FormValue("action")
	redirectTo := r.FormValue("redirect")
	if redirectTo == "" {
		redirectTo = "thumbnails"
	}

	redirectBase := "/" + redirectTo

	switch action {
	case "generate_all":
		missing, err := getMissingThumbnailVideos()
		if err != nil {
			http.Redirect(w, r, redirectBase+"?error="+url.QueryEscape("Failed to get videos: "+err.Error()), http.StatusSeeOther)
			return
		}

		successCount := 0
		var errors []string

		for _, v := range missing {
			err := generateThumbnail(v.Path, config.UploadDir, v.Filename)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", v.Filename, err))
			} else {
				successCount++
			}
		}

		if len(errors) > 0 {
			http.Redirect(w, r, redirectBase+"?success="+url.QueryEscape(fmt.Sprintf("Generated %d thumbnails", successCount))+"&error="+url.QueryEscape(fmt.Sprintf("Failed: %s", strings.Join(errors, "; "))), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, redirectBase+"?success="+url.QueryEscape(fmt.Sprintf("Successfully generated %d thumbnails", successCount)), http.StatusSeeOther)
		}

	case "generate_single":
		fileID := r.FormValue("file_id")
		timestamp := strings.TrimSpace(r.FormValue("timestamp"))

		if timestamp == "" {
			timestamp = "00:00:05"
		}

		var filename, path string
		err := db.QueryRow("SELECT filename, path FROM files WHERE id=?", fileID).Scan(&filename, &path)
		if err != nil {
			http.Redirect(w, r, redirectBase+"?error="+url.QueryEscape("File not found"), http.StatusSeeOther)
			return
		}

		err = generateThumbnailAtTime(path, config.UploadDir, filename, timestamp)
		if err != nil {
			http.Redirect(w, r, redirectBase+"?error="+url.QueryEscape("Failed to generate thumbnail: "+err.Error()), http.StatusSeeOther)
			return
		}

		if redirectTo == "admin" {
			http.Redirect(w, r, "/admin?success="+url.QueryEscape(fmt.Sprintf("Thumbnail generated for file %s at %s", fileID, timestamp)), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, fmt.Sprintf("/file/%s?success=%s", fileID, url.QueryEscape(fmt.Sprintf("Thumbnail generated at %s", timestamp))), http.StatusSeeOther)
		}

	default:
		http.Redirect(w, r, redirectBase, http.StatusSeeOther)
	}
}

func generateThumbnail(videoPath, uploadDir, filename string) error {
	thumbDir := filepath.Join(uploadDir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnails directory: %v", err)
	}

	thumbPath := filepath.Join(thumbDir, filename+".jpg")

	cmd := exec.Command("ffmpeg", "-y", "-ss", "00:00:05", "-i", videoPath, "-vframes", "1", "-vf", "scale=400:-1", thumbPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		cmd := exec.Command("ffmpeg", "-y", "-i", videoPath, "-vframes", "1", "-vf", "scale=400:-1", thumbPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("failed to generate thumbnail: %v", err2)
		}
	}

	return nil
}

func getMissingThumbnailVideos() ([]VideoFile, error) {
	allVideos, err := getVideoFiles()
	if err != nil {
		return nil, err
	}

	var missing []VideoFile
	for _, v := range allVideos {
		if !v.HasThumbnail {
			missing = append(missing, v)
		}
	}

	return missing, nil
}