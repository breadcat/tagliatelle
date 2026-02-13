package main

import (
	"net/http"
)

// RegisterRoutes sets up all HTTP routes for the application
func RegisterRoutes() {
	// Page handlers
	http.HandleFunc("/", listFilesHandler)
	http.HandleFunc("/add", uploadHandler)
	http.HandleFunc("/add-yt", ytdlpHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/bulk-tag", bulkTagHandler)
	http.HandleFunc("/cbz/", cbzViewerHandler)
	http.HandleFunc("/file/", fileRouter)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/tag/", tagFilterHandler)
	http.HandleFunc("/tags", tagsHandler)
	http.HandleFunc("/thumbnails/generate", generateThumbnailHandler)
	http.HandleFunc("/untagged", untaggedFilesHandler)
	http.HandleFunc("/upload-url", uploadFromURLHandler)
	// Static file serving
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.UploadDir))))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
}
