package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AdminPageData struct {
	Config            Config
	Error             string
	Success           string
	OrphanData        OrphanData
	ActiveTab         string
	MissingThumbnails []VideoFile
}

func renderAdminPage(w http.ResponseWriter, r *http.Request, data AdminPageData) {
	if data.ActiveTab == "" {
		data.ActiveTab = r.FormValue("active_tab")
	}
	pageData := buildPageData("Admin", data)
	renderTemplate(w, "admin.html", pageData)
}

func currentAdminState(r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) AdminPageData {
	return AdminPageData{
		Config:            config,
		OrphanData:        orphanData,
		ActiveTab:         r.FormValue("active_tab"),
		MissingThumbnails: missingThumbnails,
	}
}

func validateConfig(newConfig Config) error {
	if newConfig.GallerySize == "" {
		return fmt.Errorf("gallery size cannot be empty")
	}
	if newConfig.ItemsPerPage == "" {
		return fmt.Errorf("items per page cannot be empty")
	}
	return nil
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	// Get orphaned files
	orphanData, _ := getOrphanedFiles(config.UploadDir)

	// Get video files for thumbnails
	missingThumbnails, _ := getMissingThumbnailVideos()

	switch r.Method {
	case http.MethodPost:
		switch r.FormValue("action") {
		case "save", "":
			handleSaveSettings(w, r, orphanData, missingThumbnails)

		case "backup":
			err := backupDatabase(config.DatabasePath)
			data := currentAdminState(r, orphanData, missingThumbnails)
			data.Error = errorString(err)
			data.Success = successString(err, "Database backup created successfully!")
			renderAdminPage(w, r, data)

		case "vacuum":
			err := vacuumDatabase(config.DatabasePath)
			data := currentAdminState(r, orphanData, missingThumbnails)
			data.Error = errorString(err)
			data.Success = successString(err, "Database vacuum completed successfully!")
			renderAdminPage(w, r, data)

		case "save_aliases":
			handleSaveAliases(w, r, orphanData, missingThumbnails)

		case "save_sed_rules":
			handleSaveSedRules(w, r, orphanData, missingThumbnails)

		case "compute_properties":
			handleComputeProperties(w, r, orphanData, missingThumbnails)
		}

	default:
		renderAdminPage(w, r, currentAdminState(r, orphanData, missingThumbnails))
	}
}

func parseAliasesFromForm(r *http.Request) []TagAliasGroup {
	var groups []TagAliasGroup
	for i := 0; ; i++ {
		category := strings.TrimSpace(r.FormValue(fmt.Sprintf("aliases[%d][category]", i)))
		if category == "" {
			break
		}
		var aliases []string
		for j := 0; ; j++ {
			v := strings.TrimSpace(r.FormValue(fmt.Sprintf("aliases[%d][aliases][%d]", i, j)))
			if v == "" {
				break
			}
			aliases = append(aliases, v)
		}
		if len(aliases) >= 2 {
			groups = append(groups, TagAliasGroup{Category: category, Aliases: aliases})
		}
	}
	return groups
}

func parseSedRulesFromForm(r *http.Request) ([]SedRule, error) {
	var rules []SedRule
	for i := 0; ; i++ {
		name := strings.TrimSpace(r.FormValue(fmt.Sprintf("sed_rules[%d][name]", i)))
		if name == "" {
			break
		}
		command := strings.TrimSpace(r.FormValue(fmt.Sprintf("sed_rules[%d][command]", i)))
		if command == "" {
			return nil, fmt.Errorf("rule %s is missing a command", strconv.Itoa(i+1))
		}
		rules = append(rules, SedRule{
			Name:        name,
			Description: strings.TrimSpace(r.FormValue(fmt.Sprintf("sed_rules[%d][description]", i))),
			Command:     command,
		})
	}
	return rules, nil
}

func handleSaveAliases(w http.ResponseWriter, r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) {
	config.TagAliases = parseAliasesFromForm(r)

	if err := SaveConfig(db, config); err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = "Failed to save configuration: " + err.Error()
		renderAdminPage(w, r, data)
		return
	}

	data := currentAdminState(r, orphanData, missingThumbnails)
	data.Success = "Tag aliases saved successfully!"
	renderAdminPage(w, r, data)
}

func handleSaveSettings(w http.ResponseWriter, r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) {
	newConfig := config // preserve runtime fields
	newConfig.GallerySize = strings.TrimSpace(r.FormValue("gallery_size"))
	newConfig.ItemsPerPage = strings.TrimSpace(r.FormValue("items_per_page"))

	if err := validateConfig(newConfig); err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = err.Error()
		renderAdminPage(w, r, data)
		return
	}

	config = newConfig

	if err := SaveConfig(db, config); err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = "Failed to save configuration: " + err.Error()
		renderAdminPage(w, r, data)
		return
	}

	data := currentAdminState(r, orphanData, missingThumbnails)
	data.Success = "Settings saved successfully!"
	renderAdminPage(w, r, data)
}

func handleSaveSedRules(w http.ResponseWriter, r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) {
	rules, err := parseSedRulesFromForm(r)
	if err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = err.Error()
		renderAdminPage(w, r, data)
		return
	}

	config.SedRules = rules

	if err := SaveConfig(db, config); err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = "Failed to save configuration: " + err.Error()
		renderAdminPage(w, r, data)
		return
	}

	data := currentAdminState(r, orphanData, missingThumbnails)
	data.Success = "Sed rules saved successfully!"
	renderAdminPage(w, r, data)
}

func backupDatabase(dbPath string) error {
	if dbPath == "" {
		return fmt.Errorf("database path not configured")
	}

	backupDir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backups directory: %w", err)
	}

	dbName := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s_backup_%s.db", dbName, timestamp))

	input, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer input.Close()

	output, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	return nil
}

func vacuumDatabase(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if _, err = db.Exec("VACUUM;"); err != nil {
		return fmt.Errorf("VACUUM failed: %w", err)
	}

	return nil
}

func getFilesInDB() (map[string]bool, error) {
	rows, err := db.Query("SELECT filename FROM files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fileMap := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		fileMap[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return fileMap, nil
}
