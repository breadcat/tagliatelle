package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db     *sql.DB
	tmpl   *template.Template
	config Config
)

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var err error
	db, err = sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT,
		path TEXT,
		description TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE
	);
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER,
		value TEXT,
		UNIQUE(category_id, value)
	);
	CREATE TABLE IF NOT EXISTS file_tags (
		file_id INTEGER,
		tag_id INTEGER,
		UNIQUE(file_id, tag_id)
	);
	`)
	if err != nil {
		log.Fatal(err)
	}

	os.MkdirAll(config.UploadDir, 0755)
	os.MkdirAll("static", 0755)

	tmpl = template.Must(template.New("").Funcs(template.FuncMap{
	    "add": func(a, b int) int { return a + b },
	    "sub": func(a, b int) int { return a - b },
		"hasAnySuffix": func(s string, suffixes ...string) bool {
			for _, suf := range suffixes {
				if strings.HasSuffix(strings.ToLower(s), suf) {
					return true
				}
			}
			return false
		},
    "dict": func(values ...interface{}) (map[string]interface{}, error) {
        if len(values)%2 != 0 {
            return nil, fmt.Errorf("dict requires an even number of args")
        }
        dict := make(map[string]interface{}, len(values)/2)
        for i := 0; i < len(values); i += 2 {
            key, ok := values[i].(string)
            if !ok {
                return nil, fmt.Errorf("dict keys must be strings")
            }
            dict[key] = values[i+1]
        }
        return dict, nil
    },
	}).ParseGlob("templates/*.html"))

	http.HandleFunc("/", listFilesHandler)
	http.HandleFunc("/add", uploadHandler)
	http.HandleFunc("/add-yt", ytdlpHandler)
	http.HandleFunc("/upload-url", uploadFromURLHandler)
	http.HandleFunc("/file/", fileRouter)
	http.HandleFunc("/tags", tagsHandler)
	http.HandleFunc("/tag/", tagFilterHandler)
	http.HandleFunc("/untagged", untaggedFilesHandler)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/bulk-tag", bulkTagHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/thumbnails/generate", generateThumbnailHandler)
	http.HandleFunc("/cbz/", cbzViewerHandler)

	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.UploadDir))))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Printf("Server started at http://localhost%s", config.ServerPort)
	log.Printf("Database: %s", config.DatabasePath)
	log.Printf("Upload directory: %s", config.UploadDir)
	http.ListenAndServe(config.ServerPort, nil)
}
