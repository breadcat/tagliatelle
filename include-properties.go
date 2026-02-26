package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func computeProperties(fileID int64, filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))

	setProperty(fileID, "filetype", strings.TrimPrefix(ext, "."))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		computeImageProperties(fileID, filePath)
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v":
		computeVideoProperties(fileID, filePath)
	}
}

func setProperty(fileID int64, key, value string) {
	if value == "" {
		return
	}
	_, err := db.Exec(
		`INSERT OR IGNORE INTO file_properties (file_id, key, value) VALUES (?, ?, ?)`,
		fileID, key, value,
	)
	if err != nil {
		log.Printf("Warning: failed to set property %s for file %d: %v", key, fileID, err)
	}
}

func computeImageProperties(fileID int64, filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Warning: could not open image for properties %s: %v", filePath, err)
		return
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		log.Printf("Warning: could not decode image config %s: %v", filePath, err)
		return
	}

	w, h := cfg.Width, cfg.Height

	var orientation string
	switch {
	case w > h:
		orientation = "landscape"
	case h > w:
		orientation = "portrait"
	default:
		orientation = "square"
	}
	setProperty(fileID, "orientation", orientation)

	mp := w * h
	var tier string
	switch {
	case mp < 1_000_000:
		tier = "small"
	case mp < 4_000_000:
		tier = "medium"
	case mp < 12_000_000:
		tier = "large"
	default:
		tier = "huge"
	}
	setProperty(fileID, "resolution", tier)
}

func computeVideoProperties(fileID int64, filePath string) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=nokey=1:noprint_wrappers=1",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: ffprobe failed for %s: %v", filePath, err)
		return
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		log.Printf("Warning: could not parse duration for %s: %v", filePath, err)
		return
	}

	var bucket string
	switch {
	case seconds < 60:
		bucket = "tiny"
	case seconds < 300:
		bucket = "short"
	case seconds < 2700:
		bucket = "moderate"
	default:
		bucket = "long"
	}
	setProperty(fileID, "duration", bucket)
}

func computeMissingProperties() (int, error) {
    rows, err := db.Query(`
        SELECT f.id, f.path
        FROM files f
        WHERE NOT EXISTS (
            SELECT 1 FROM file_properties fp WHERE fp.file_id = f.id
        )
    `)
    if err != nil {
        return 0, fmt.Errorf("failed to query files: %w", err)
    }

    type fileRow struct {
        id   int64
        path string
    }
    var files []fileRow
    for rows.Next() {
        var r fileRow
        if err := rows.Scan(&r.id, &r.path); err != nil {
            continue
        }
        files = append(files, r)
    }
    rows.Close()

    if err := rows.Err(); err != nil {
        return 0, err
    }

    for _, f := range files {
        computeProperties(f.id, f.path)
    }
    return len(files), nil
}

func getPropertyNav() (map[string][]PropertyDisplay, error) {
	rows, err := db.Query(`
		SELECT key, value, COUNT(*) as cnt
		FROM file_properties
		GROUP BY key, value
		ORDER BY key, value
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	propMap := make(map[string][]PropertyDisplay)
	for rows.Next() {
		var key, val string
		var count int
		if err := rows.Scan(&key, &val, &count); err != nil {
			continue
		}
		propMap[key] = append(propMap[key], PropertyDisplay{Value: val, Count: count})
	}
	return propMap, nil
}

func propertyFilterHandler(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/property/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		renderError(w, "Invalid property filter path", http.StatusBadRequest)
		return
	}

	key := parts[0]
	value := parts[1]

	page := pageFromRequest(r)
	perPage := perPageFromConfig(50)

	var total int
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT f.id)
		FROM files f
		JOIN file_properties fp ON fp.file_id = f.id
		WHERE fp.key = ? AND fp.value = ?
	`, key, value).Scan(&total)
	if err != nil {
		renderError(w, "Failed to count files", http.StatusInternalServerError)
		return
	}

	offset := (page - 1) * perPage
	files, err := queryFilesWithTags(`
		SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description
		FROM files f
		JOIN file_properties fp ON fp.file_id = f.id
		WHERE fp.key = ? AND fp.value = ?
		ORDER BY f.id DESC
		LIMIT ? OFFSET ?
	`, key, value, perPage, offset)
	if err != nil {
		renderError(w, "Failed to fetch files", http.StatusInternalServerError)
		return
	}

	breadcrumbs := []Breadcrumb{
		{Name: "home", URL: "/"},
		{Name: "properties", URL: "/properties"},
		{Name: key, URL: "/properties#prop-" + key},
		{Name: value, URL: r.URL.Path},
	}

	title := fmt.Sprintf("%s: %s", key, value)
	pageData := buildPageDataWithPagination(title, ListData{
		Tagged:      files,
		Breadcrumbs: breadcrumbs,
	}, page, total, perPage, r)
	pageData.Breadcrumbs = breadcrumbs

	renderTemplate(w, "list.html", pageData)
}

func propertiesIndexHandler(w http.ResponseWriter, r *http.Request) {
	pageData := buildPageData("Properties", nil)
	pageData.Data = pageData.Properties
	renderTemplate(w, "properties.html", pageData)
}

func handleComputeProperties(w http.ResponseWriter, r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) {
	count, err := computeMissingProperties()
	data := currentAdminState(r, orphanData, missingThumbnails)
	if err != nil {
		data.Error = "Property computation failed: " + err.Error()
	} else {
		data.Success = fmt.Sprintf("Computed properties for %d files.", count)
	}
	renderAdminPage(w, r, data)
}
