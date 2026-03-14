package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var (
	db     *sql.DB
	tmpl   *template.Template
	config Config
)

func main() {
	// CLI flags
	dataDir := flag.String("d", ".", "Data directory (stores tagliatelle.db and uploads/ subfolder)")
	port := flag.String("p", "8080", "Port to listen on")
	flag.Parse()

	// Derive paths from -d
	dbPath := filepath.Join(*dataDir, "tagliatelle.db")
	uploadDir := filepath.Join(*dataDir, "uploads")
	serverPort := fmt.Sprintf(":%s", *port)

	// Create necessary directories
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}
	os.MkdirAll("static", 0755)

	// Initialize database
	var err error
	db, err = InitDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Load config from database (gallery size, items per page, aliases, sed rules)
	config, err = LoadConfig(db)
	if err != nil {
		log.Fatalf("Failed to load config from database: %v", err)
	}

	// Inject runtime values (not stored in DB)
	config.DatabasePath = dbPath
	config.UploadDir = uploadDir
	config.ServerPort = serverPort

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
