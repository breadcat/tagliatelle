package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"tagliatelle/internal/app"
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
	app.DB, err = app.InitDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer app.DB.Close()

	// Load config from database (gallery size, items per page, aliases, sed rules)
	app.Cfg, err = app.LoadConfig(app.DB)
	if err != nil {
		log.Fatalf("Failed to load config from database: %v", err)
	}

	// Inject runtime values (not stored in DB)
	app.Cfg.DatabasePath = dbPath
	app.Cfg.UploadDir = uploadDir
	app.Cfg.ServerPort = serverPort

	// Initialize templates
	app.Tmpl, err = app.InitTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Register all routes
	app.RegisterRoutes()

	// Start server
	log.Printf("Server started at http://localhost%s", app.Cfg.ServerPort)
	log.Printf("Database: %s", app.Cfg.DatabasePath)
	log.Printf("Upload directory: %s", app.Cfg.UploadDir)

	if err := http.ListenAndServe(app.Cfg.ServerPort, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
