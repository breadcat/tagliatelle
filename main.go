package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
)

var (
	db     *sql.DB
	tmpl   *template.Template
	config Config
)

func main() {
	// Load configuration
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	var err error
	db, err = InitDatabase(config.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create necessary directories
	os.MkdirAll(config.UploadDir, 0755)
	os.MkdirAll("static", 0755)

	// Initialize templates
	tmpl, err = InitTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Register all routes
	RegisterRoutes()

	// Start server
	log.Printf("Server started at http://localhost%s", config.ServerPort)
	log.Printf("Database: %s", config.DatabasePath)
	log.Printf("Upload directory: %s", config.UploadDir)
	
	if err := http.ListenAndServe(config.ServerPort, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
