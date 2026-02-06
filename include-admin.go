package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
)

func loadConfig() error {
	config = Config{
		DatabasePath: "./database.db",
		UploadDir:    "uploads",
		ServerPort:   ":8080",
		InstanceName: "Tagliatelle",
		GallerySize:  "400px",
		ItemsPerPage: "100",
		TagAliases:   []TagAliasGroup{},
	}

	if data, err := ioutil.ReadFile("config.json"); err == nil {
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
	return ioutil.WriteFile("config.json", data, 0644)
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
	orphans, _ := getOrphanedFiles(config.UploadDir)

	// Get video files for thumbnails
	missingThumbnails, _ := getMissingThumbnailVideos()

	switch r.Method {
	case http.MethodPost:
		action := r.FormValue("action")

		switch action {
		case "save", "":
			handleSaveSettings(w, r, orphans, missingThumbnails)
			return

		case "backup":
			err := backupDatabase(config.DatabasePath)
			pageData := buildPageData("Admin", struct {
				Config            Config
				Error             string
				Success           string
				Orphans           []string
				MissingThumbnails []VideoFile
			}{
				Config:            config,
				Error:             errorString(err),
				Success:           successString(err, "Database backup created successfully!"),
				Orphans:           orphans,
				MissingThumbnails: missingThumbnails,
			})
			renderTemplate(w, "admin.html", pageData)
			return

		case "vacuum":
			err := vacuumDatabase(config.DatabasePath)
			pageData := buildPageData("Admin", struct {
				Config            Config
				Error             string
				Success           string
				Orphans           []string
				MissingThumbnails []VideoFile
			}{
				Config:            config,
				Error:             errorString(err),
				Success:           successString(err, "Database vacuum completed successfully!"),
				Orphans:           orphans,
				MissingThumbnails: missingThumbnails,
			})
			renderTemplate(w, "admin.html", pageData)
			return

		case "save_aliases":
			handleSaveAliases(w, r, orphans, missingThumbnails)
			return
		}

	default:
		pageData := buildPageData("Admin", struct {
			Config            Config
			Error             string
			Success           string
			Orphans           []string
			MissingThumbnails []VideoFile
		}{
			Config:            config,
			Error:             "",
			Success:           "",
			Orphans:           orphans,
			MissingThumbnails: missingThumbnails,
		})
		renderTemplate(w, "admin.html", pageData)
	}
}

func handleSaveAliases(w http.ResponseWriter, r *http.Request, orphans []string, missingThumbnails []VideoFile) {
	aliasesJSON := r.FormValue("aliases_json")

	var aliases []TagAliasGroup
	if aliasesJSON != "" {
		if err := json.Unmarshal([]byte(aliasesJSON), &aliases); err != nil {
			pageData := buildPageData("Admin", struct {
				Config            Config
				Error             string
				Success           string
				Orphans           []string
				MissingThumbnails []VideoFile
			}{
				Config:            config,
				Error:             "Invalid aliases JSON: " + err.Error(),
				Success:           "",
				Orphans:           orphans,
				MissingThumbnails: missingThumbnails,
			})
			renderTemplate(w, "admin.html", pageData)
			return
		}
	}

	config.TagAliases = aliases

	if err := saveConfig(); err != nil {
		pageData := buildPageData("Admin", struct {
			Config            Config
			Error             string
			Success           string
			Orphans           []string
			MissingThumbnails []VideoFile
		}{
			Config:            config,
			Error:             "Failed to save configuration: " + err.Error(),
			Success:           "",
			Orphans:           orphans,
			MissingThumbnails: missingThumbnails,
		})
		renderTemplate(w, "admin.html", pageData)
		return
	}

	pageData := buildPageData("Admin", struct {
		Config            Config
		Error             string
		Success           string
		Orphans           []string
		MissingThumbnails []VideoFile
	}{
		Config:            config,
		Error:             "",
		Success:           "Tag aliases saved successfully!",
		Orphans:           orphans,
		MissingThumbnails: missingThumbnails,
	})
	renderTemplate(w, "admin.html", pageData)
}

func handleSaveSettings(w http.ResponseWriter, r *http.Request, orphans []string, missingThumbnails []VideoFile) {
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
		pageData := buildPageData("Admin", struct {
			Config            Config
			Error             string
			Success           string
			Orphans           []string
			MissingThumbnails []VideoFile
		}{
			Config:            config,
			Error:             err.Error(),
			Success:           "",
			Orphans:           orphans,
			MissingThumbnails: missingThumbnails,
		})
		renderTemplate(w, "admin.html", pageData)
		return
	}

	needsRestart := (newConfig.DatabasePath != config.DatabasePath ||
		newConfig.ServerPort != config.ServerPort)

	config = newConfig
	if err := saveConfig(); err != nil {
		pageData := buildPageData("Admin", struct {
			Config            Config
			Error             string
			Success           string
			Orphans           []string
			MissingThumbnails []VideoFile
		}{
			Config:            config,
			Error:             "Failed to save configuration: " + err.Error(),
			Success:           "",
			Orphans:           orphans,
			MissingThumbnails: missingThumbnails,
		})
		renderTemplate(w, "admin.html", pageData)
		return
	}

	var message string
	if needsRestart {
		message = "Settings saved successfully! Please restart the server for database/port changes to take effect."
	} else {
		message = "Settings saved successfully!"
	}

	pageData := buildPageData("Admin", struct {
		Config            Config
		Error             string
		Success           string
		Orphans           []string
		MissingThumbnails []VideoFile
	}{
		Config:            config,
		Error:             "",
		Success:           message,
		Orphans:           orphans,
		MissingThumbnails: missingThumbnails,
	})
	renderTemplate(w, "admin.html", pageData)
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

	_, err = db.Exec("VACUUM;")
	if err != nil {
		return fmt.Errorf("VACUUM failed: %w", err)
	}

	return nil
}

func getFilesInDB() (map[string]bool, error) {
	rows, err := db.Query(`SELECT filename FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fileMap := make(map[string]bool)
	for rows.Next() {
		var name string
		rows.Scan(&name)
		fileMap[name] = true
	}
	return fileMap, nil
}
