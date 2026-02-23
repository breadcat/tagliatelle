package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pageData := buildPageData("Add File", nil)
		renderTemplate(w, "add.html", pageData)
		return
	}

	// Parse the multipart form (with max memory limit, e.g., 32MB)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		renderError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		renderError(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	var warnings []string
	var lastID int64

	// Process each file
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			renderError(w, "Failed to open uploaded file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		id, warningMsg, err := processUpload(file, fileHeader.Filename)
		if err != nil {
			renderError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		lastID = id

		if warningMsg != "" {
			warnings = append(warnings, warningMsg)
		}
	}

	var warningMsg string
	if len(warnings) > 0 {
		warningMsg = strings.Join(warnings, "; ")
	}

	redirectTarget := "/untagged"
	if len(files) == 1 && lastID != 0 {
		redirectTarget = "/file/" + strconv.FormatInt(lastID, 10)
	}

	redirectWithWarning(w, r, redirectTarget, warningMsg)
}

func processUpload(src io.Reader, filename string) (int64, string, error) {
    finalFilename, finalPath, err := checkFileConflictStrict(filename)
    if err != nil {
        return 0, "", err
    }

    tempPath := finalPath + ".tmp"
    tempFile, err := os.Create(tempPath)
    if err != nil {
        return 0, "", fmt.Errorf("failed to create temp file: %v", err)
    }

    _, err = io.Copy(tempFile, src)
    tempFile.Close()
    if err != nil {
        os.Remove(tempPath)
        return 0, "", fmt.Errorf("failed to copy file data: %v", err)
    }

    ext := strings.ToLower(filepath.Ext(filename))
    videoExts := map[string]bool{
        ".mp4": true, ".mov": true, ".avi": true,
        ".mkv": true, ".webm": true, ".m4v": true,
    }

    var processedPath string
    var warningMsg string

    if videoExts[ext] || ext == ".cbz" {
        // Process videos and CBZ files
        processedPath, warningMsg, err = processVideoFile(tempPath, finalPath)
        if err != nil {
            os.Remove(tempPath)
            return 0, "", err
        }
    } else {
        // Non-video, non-CBZ → just rename temp file to final
        if err := os.Rename(tempPath, finalPath); err != nil {
            return 0, "", fmt.Errorf("failed to move file: %v", err)
        }
        processedPath = finalPath
    }

    id, err := saveFileToDatabase(finalFilename, processedPath)
    if err != nil {
        os.Remove(processedPath)
        return 0, "", err
    }

    return id, warningMsg, nil
}

func uploadFromURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}

	fileURL := r.FormValue("fileurl")
	if fileURL == "" {
		renderError(w, "No URL provided", http.StatusBadRequest)
		return
	}

	customFilename := strings.TrimSpace(r.FormValue("filename"))

	parsedURL, err := url.ParseRequestURI(fileURL)
	if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
		renderError(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	resp, err := http.Get(fileURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		renderError(w, "Failed to download file", http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	var filename string
	urlExt := filepath.Ext(parsedURL.Path)
	if customFilename != "" {
		filename = customFilename
		if filepath.Ext(filename) == "" && urlExt != "" {
			filename += urlExt
		}
	} else {
		parts := strings.Split(parsedURL.Path, "/")
		filename = parts[len(parts)-1]
		if filename == "" {
			filename = "file_from_url"
		}
	}

	id, warningMsg, err := processUpload(resp.Body, filename)
	if err != nil {
		renderError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirectWithWarning(w, r, fmt.Sprintf("/file/%d", id), warningMsg)
}

func ytdlpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}

	videoURL := r.FormValue("url")
	if videoURL == "" {
		renderError(w, "No URL provided", http.StatusBadRequest)
		return
	}

	outTemplate := filepath.Join(config.UploadDir, "%(title)s.%(ext)s")
	filenameCmd := exec.Command("yt-dlp", "--playlist-items", "1", "-f", "mp4", "-o", outTemplate, "--get-filename", videoURL)
	filenameBytes, err := filenameCmd.Output()
	if err != nil {
		renderError(w, fmt.Sprintf("Failed to get filename: %v", err), http.StatusInternalServerError)
		return
	}
	expectedFullPath := strings.TrimSpace(string(filenameBytes))
	expectedFilename := filepath.Base(expectedFullPath)

	finalFilename, finalPath, err := checkFileConflictStrict(expectedFilename)
	if err != nil {
		renderError(w, err.Error(), http.StatusConflict)
		return
	}

	downloadCmd := exec.Command("yt-dlp", "--playlist-items", "1", "-f", "mp4", "-o", outTemplate, videoURL)
	downloadCmd.Stdout = os.Stdout
	downloadCmd.Stderr = os.Stderr
	if err := downloadCmd.Run(); err != nil {
		renderError(w, fmt.Sprintf("Failed to download video: %v", err), http.StatusInternalServerError)
		return
	}

	if expectedFullPath != finalPath {
		if err := os.Rename(expectedFullPath, finalPath); err != nil {
			renderError(w, fmt.Sprintf("Failed to move downloaded file: %v", err), http.StatusInternalServerError)
			return
		}
	}

	tempPath := finalPath + ".tmp"
	if err := os.Rename(finalPath, tempPath); err != nil {
		renderError(w, fmt.Sprintf("Failed to create temp file for processing: %v", err), http.StatusInternalServerError)
		return
	}

	processedPath, warningMsg, err := processVideoFile(tempPath, finalPath)
	if err != nil {
		os.Remove(tempPath)
		renderError(w, fmt.Sprintf("Failed to process video: %v", err), http.StatusInternalServerError)
		return
	}

	id, err := saveFileToDatabase(finalFilename, processedPath)
	if err != nil {
		os.Remove(processedPath)
		renderError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirectWithWarning(w, r, fmt.Sprintf("/file/%d", id), warningMsg)
}

func processVideoFile(tempPath, finalPath string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(finalPath))

	// Handle CBZ files
	if ext == ".cbz" {
		if err := os.Rename(tempPath, finalPath); err != nil {
			return "", "", fmt.Errorf("failed to move file: %v", err)
		}
		if err := generateCBZThumbnail(finalPath, config.UploadDir, filepath.Base(finalPath)); err != nil {
			log.Printf("Warning: could not generate CBZ thumbnail: %v", err)
		}
		return finalPath, "", nil
	}

	// Handle video files
	codec, err := detectVideoCodec(tempPath)
	if err != nil {
		return "", "", err
	}
	if codec == "hevc" || codec == "h265" {
		warningMsg := "The video uses HEVC and has been re-encoded to H.264 for browser compatibility."
		if err := reencodeHEVCToH264(tempPath, finalPath); err != nil {
			return "", "", fmt.Errorf("failed to re-encode HEVC video: %v", err)
		}
		os.Remove(tempPath)
		return finalPath, warningMsg, nil
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", "", fmt.Errorf("failed to move file: %v", err)
	}

	// Generate thumbnail for video files
	if ext == ".mp4" || ext == ".mov" || ext == ".avi" || ext == ".mkv" || ext == ".webm" || ext == ".m4v" {
		if err := generateThumbnail(finalPath, config.UploadDir, filepath.Base(finalPath)); err != nil {
			log.Printf("Warning: could not generate thumbnail: %v", err)
		}
	}

	return finalPath, "", nil
}

func saveFileToDatabase(filename, path string) (int64, error) {
	res, err := db.Exec("INSERT INTO files (filename, path, description) VALUES (?, ?, '')", filename, path)
	if err != nil {
		return 0, fmt.Errorf("failed to save file to database: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get inserted ID: %v", err)
	}
	return id, nil
}


func detectVideoCodec(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name", "-of", "default=nokey=1:noprint_wrappers=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to probe video codec: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func reencodeHEVCToH264(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg", "-i", inputPath,
		"-c:v", "libx264", "-profile:v", "baseline", "-preset", "fast", "-crf", "23",
		"-c:a", "aac", "-movflags", "+faststart", outputPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}