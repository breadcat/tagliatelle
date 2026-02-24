package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func loadConfig() error {
	config = Config{
		DatabasePath: "./database.db",
		UploadDir:    "uploads",
		ServerPort:   ":8080",
		InstanceName: "Tagliatelle",
		GallerySize:  "400px",
		ItemsPerPage: "100",
		TagAliases:   []TagAliasGroup{},
		SedRules:     []SedRule{},
	}

	if data, err := os.ReadFile("config.json"); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}
	}

	return os.MkdirAll(config.UploadDir, 0755)
}

func saveConfig() error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", data, 0644)
}

func validateConfig(newConfig Config) error {
	if newConfig.DatabasePath == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if newConfig.UploadDir == "" {
		return fmt.Errorf("upload directory cannot be empty")
	}

	if newConfig.ServerPort == "" || !strings.HasPrefix(newConfig.ServerPort, ":") {
		return fmt.Errorf("server port must be in format ':8080'")
	}

	if err := os.MkdirAll(newConfig.UploadDir, 0755); err != nil {
		return fmt.Errorf("cannot create upload directory: %v", err)
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
		}

	default:
		renderAdminPage(w, r, currentAdminState(r, orphanData, missingThumbnails))
	}
}

func handleSaveAliases(w http.ResponseWriter, r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) {
	aliasesJSON := r.FormValue("aliases_json")

	var aliases []TagAliasGroup
	if aliasesJSON != "" {
		if err := json.Unmarshal([]byte(aliasesJSON), &aliases); err != nil {
			data := currentAdminState(r, orphanData, missingThumbnails)
			data.Error = "Invalid aliases JSON: " + err.Error()
			renderAdminPage(w, r, data)
			return
		}
	}

	config.TagAliases = aliases

	if err := saveConfig(); err != nil {
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
	newConfig := Config{
		DatabasePath: strings.TrimSpace(r.FormValue("database_path")),
		UploadDir:    strings.TrimSpace(r.FormValue("upload_dir")),
		ServerPort:   strings.TrimSpace(r.FormValue("server_port")),
		InstanceName: strings.TrimSpace(r.FormValue("instance_name")),
		GallerySize:  strings.TrimSpace(r.FormValue("gallery_size")),
		ItemsPerPage: strings.TrimSpace(r.FormValue("items_per_page")),
		TagAliases:   config.TagAliases, // Preserve existing aliases
	}

	if err := validateConfig(newConfig); err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = err.Error()
		renderAdminPage(w, r, data)
		return
	}

	needsRestart := newConfig.DatabasePath != config.DatabasePath ||
		newConfig.ServerPort != config.ServerPort

	config = newConfig

	if err := saveConfig(); err != nil {
		data := currentAdminState(r, orphanData, missingThumbnails)
		data.Error = "Failed to save configuration: " + err.Error()
		renderAdminPage(w, r, data)
		return
	}

	data := currentAdminState(r, orphanData, missingThumbnails)
	if needsRestart {
		data.Success = "Settings saved successfully! Please restart the server for database/port changes to take effect."
	} else {
		data.Success = "Settings saved successfully!"
	}
	renderAdminPage(w, r, data)
}

func handleSaveSedRules(w http.ResponseWriter, r *http.Request, orphanData OrphanData, missingThumbnails []VideoFile) {
	sedRulesJSON := r.FormValue("sed_rules_json")

	var sedRules []SedRule
	if sedRulesJSON != "" {
		if err := json.Unmarshal([]byte(sedRulesJSON), &sedRules); err != nil {
			data := currentAdminState(r, orphanData, missingThumbnails)
			data.Error = "Invalid sed rules JSON: " + err.Error()
			renderAdminPage(w, r, data)
			return
		}
	}

	config.SedRules = sedRules

	if err := saveConfig(); err != nil {
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

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s_backup_%s.db", strings.TrimSuffix(dbPath, filepath.Ext(dbPath)), timestamp)

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
