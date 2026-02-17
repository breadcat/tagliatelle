package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// GetNotes retrieves the notes content from database
func GetNotes(db *sql.DB) (string, error) {
	var content string
	err := db.QueryRow("SELECT content FROM notes WHERE id = 1").Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

// SaveNotes saves the notes content to database with sorting and deduplication
func SaveNotes(db *sql.DB, content string) error {
	// Process: deduplicate and sort
	processed := ProcessNotes(content)

	_, err := db.Exec(`
		INSERT INTO notes (id, content, updated_at)
		VALUES (1, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			updated_at = excluded.updated_at
	`, processed)

	return err
}

// ProcessNotes deduplicates and sorts lines alphabetically
func ProcessNotes(content string) string {
	lines := strings.Split(content, "\n")

	// Deduplicate using map
	seen := make(map[string]bool)
	var unique []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // Skip empty lines
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			unique = append(unique, trimmed)
		}
	}

	// Sort alphabetically (case-insensitive)
	sort.Slice(unique, func(i, j int) bool {
		return strings.ToLower(unique[i]) < strings.ToLower(unique[j])
	})

	return strings.Join(unique, "\n")
}

// ApplySedRule applies a sed command to content
func ApplySedRule(content, sedCmd string) (string, error) {
	// Create temp file for input
	tmpIn, err := os.CreateTemp("", "notes-in-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpIn.Name())
	defer tmpIn.Close()

	if _, err := tmpIn.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to write temp file: %v", err)
	}
	tmpIn.Close()

	// Run sed command
	cmd := exec.Command("sed", sedCmd, tmpIn.Name())

	// Capture both stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Include stderr in error message for debugging
		errMsg := stderr.String()
		if errMsg != "" {
			return "", fmt.Errorf("sed failed: %s (command: sed %s)", errMsg, sedCmd)
		}
		return "", fmt.Errorf("sed failed: %v (command: sed %s)", err, sedCmd)
	}

	return stdout.String(), nil
}

// ParseNote parses a line into category and value
func ParseNote(line string) Note {
	parts := strings.SplitN(line, ">", 2)

	note := Note{Original: line}

	if len(parts) == 2 {
		note.Category = strings.TrimSpace(parts[0])
		note.Value = strings.TrimSpace(parts[1])
	} else {
		note.Value = strings.TrimSpace(line)
	}

	return note
}

// FilterNotes filters notes by search term
func FilterNotes(content, searchTerm string) string {
	if searchTerm == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	var filtered []string

	searchLower := strings.ToLower(searchTerm)

	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), searchLower) {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
}

// FilterByCategory filters notes by category
func FilterByCategory(content, category string) string {
	if category == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	var filtered []string

	for _, line := range lines {
		note := ParseNote(line)
		if note.Category == category {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
}

// GetCategories returns a sorted list of unique categories
func GetCategories(content string) []string {
	lines := strings.Split(content, "\n")
	categoryMap := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		note := ParseNote(trimmed)
		if note.Category != "" {
			categoryMap[note.Category] = true
		}
	}

	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}

	sort.Strings(categories)
	return categories
}

// GetNoteStats returns statistics about the notes
func GetNoteStats(content string) map[string]int {
	lines := strings.Split(content, "\n")

	totalLines := 0
	categorizedLines := 0
	categories := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		totalLines++
		note := ParseNote(trimmed)

		if note.Category != "" {
			categorizedLines++
			categories[note.Category] = true
		}
	}

	return map[string]int{
		"total_lines":       totalLines,
		"categorized_lines": categorizedLines,
		"uncategorized":     totalLines - categorizedLines,
		"unique_categories": len(categories),
	}
}

// CountLines returns the number of non-empty lines
func CountLines(content string) int {
	lines := strings.Split(content, "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// notesViewHandler displays the notes editor page
func notesViewHandler(w http.ResponseWriter, r *http.Request) {
	content, err := GetNotes(db)
	if err != nil {
		http.Error(w, "Failed to load notes", http.StatusInternalServerError)
		log.Printf("Error loading notes: %v", err)
		return
	}

	stats := GetNoteStats(content)
	categories := GetCategories(content)

	notesData := struct {
		Content    string
		Stats      map[string]int
		Categories []string
		SedRules   []SedRule
	}{
		Content:    content,
		Stats:      stats,
		Categories: categories,
		SedRules:   config.SedRules,
	}

	pageData := buildPageData("Notes", notesData)

	if err := tmpl.ExecuteTemplate(w, "notes.html", pageData); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
	}
}

// notesSaveHandler saves the notes content
func notesSaveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	// Process (deduplicate and sort) before saving
	if err := SaveNotes(db, content); err != nil {
		http.Error(w, "Failed to save notes", http.StatusInternalServerError)
		log.Printf("Error saving notes: %v", err)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Notes saved successfully",
	})
}

// notesFilterHandler filters notes by search term or category
func notesFilterHandler(w http.ResponseWriter, r *http.Request) {
	content, err := GetNotes(db)
	if err != nil {
		http.Error(w, "Failed to load notes", http.StatusInternalServerError)
		return
	}

	searchTerm := r.URL.Query().Get("search")
	category := r.URL.Query().Get("category")

	// Filter by search term
	if searchTerm != "" {
		content = FilterNotes(content, searchTerm)
	}

	// Filter by category
	if category != "" {
		content = FilterByCategory(content, category)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(content))
}

// notesStatsHandler returns statistics about the notes
func notesStatsHandler(w http.ResponseWriter, r *http.Request) {
	content, err := GetNotes(db)
	if err != nil {
		http.Error(w, "Failed to load notes", http.StatusInternalServerError)
		return
	}

	stats := GetNoteStats(content)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// notesApplySedHandler applies a sed rule to the notes
func notesApplySedHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("notesApplySedHandler called - Method: %s, Path: %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		log.Printf("Wrong method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	content := r.FormValue("content")
	ruleIndexStr := r.FormValue("rule_index")
	log.Printf("Received: content length=%d, rule_index=%s", len(content), ruleIndexStr)

	ruleIndex, err := strconv.Atoi(ruleIndexStr)
	if err != nil || ruleIndex < 0 || ruleIndex >= len(config.SedRules) {
		log.Printf("Invalid rule index: %s (error: %v, len(rules)=%d)", ruleIndexStr, err, len(config.SedRules))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid rule index",
		})
		return
	}

	rule := config.SedRules[ruleIndex]
	log.Printf("Applying rule: %s (command: %s)", rule.Name, rule.Command)
	result, err := ApplySedRule(content, rule.Command)
	if err != nil {
		log.Printf("Sed rule error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return the processed content
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"content": result,
		"stats":   GetNoteStats(result),
	}
	log.Printf("Sed rule success, returning %d bytes", len(result))
	json.NewEncoder(w).Encode(response)
}

// notesPreviewHandler previews an operation without saving
func notesPreviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content := r.FormValue("content")

	// Process (deduplicate and sort)
	processed := ProcessNotes(content)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"content":   processed,
		"stats":     GetNoteStats(processed),
		"lineCount": CountLines(processed),
	})
}

// notesExportHandler exports notes as plain text file
func notesExportHandler(w http.ResponseWriter, r *http.Request) {
	content, err := GetNotes(db)
	if err != nil {
		http.Error(w, "Failed to load notes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=notes.txt")
	w.Write([]byte(content))
}

// notesImportHandler imports notes from uploaded file
func notesImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, file); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	content := buf.String()

	// Option to merge or replace
	mergeMode := r.FormValue("merge") == "true"

	if mergeMode {
		// Merge with existing content
		existingContent, _ := GetNotes(db)
		content = existingContent + "\n" + content
	}

	// Save (will auto-process)
	if err := SaveNotes(db, content); err != nil {
		http.Error(w, "Failed to save notes", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/notes", http.StatusSeeOther)
}
