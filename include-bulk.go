package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func applyBulkTagOperations(fileIDs []int, category, value, operation string) error {
	category = strings.TrimSpace(category)
	value = strings.TrimSpace(value)
	if category == "" {
		return fmt.Errorf("category cannot be empty")
	}

	if operation == "add" && value == "" {
		return fmt.Errorf("value cannot be empty when adding tags")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	var catID int
	err = tx.QueryRow("SELECT id FROM categories WHERE name=?", category).Scan(&catID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to query category: %v", err)
	}

	if catID == 0 {
		if operation == "remove" {
			return fmt.Errorf("cannot remove non-existent category: %s", category)
		}
		res, err := tx.Exec("INSERT INTO categories(name) VALUES(?)", category)
		if err != nil {
			return fmt.Errorf("failed to create category: %v", err)
		}
		cid, _ := res.LastInsertId()
		catID = int(cid)
	}

	var tagID int
	if value != "" {
		err = tx.QueryRow("SELECT id FROM tags WHERE category_id=? AND value=?", catID, value).Scan(&tagID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to query tag: %v", err)
		}

		if tagID == 0 {
			if operation == "remove" {
				return fmt.Errorf("cannot remove non-existent tag: %s=%s", category, value)
			}
			res, err := tx.Exec("INSERT INTO tags(category_id, value) VALUES(?, ?)", catID, value)
			if err != nil {
				return fmt.Errorf("failed to create tag: %v", err)
			}
			tid, _ := res.LastInsertId()
			tagID = int(tid)
		}
	}

	for _, fileID := range fileIDs {
		if operation == "add" {
			_, err = tx.Exec("INSERT OR IGNORE INTO file_tags(file_id, tag_id) VALUES (?, ?)", fileID, tagID)
		} else if operation == "remove" {
			if value != "" {
				_, err = tx.Exec("DELETE FROM file_tags WHERE file_id=? AND tag_id=?", fileID, tagID)
			} else {
				_, err = tx.Exec(`DELETE FROM file_tags WHERE file_id=? AND tag_id IN (SELECT t.id FROM tags t WHERE t.category_id=?)`, fileID, catID)
			}
		} else {
			return fmt.Errorf("invalid operation: %s (must be 'add' or 'remove')", operation)
		}
		if err != nil {
			return fmt.Errorf("failed to %s tag for file %d: %v", operation, fileID, err)
		}
	}

	return tx.Commit()
}

func getBulkTagFormData() BulkTagFormData {
	catRows, _ := db.Query("SELECT name FROM categories ORDER BY name")
	var cats []string
	for catRows.Next() {
		var c string
		catRows.Scan(&c)
		cats = append(cats, c)
	}
	catRows.Close()

	recentRows, _ := db.Query("SELECT id, filename FROM files ORDER BY id DESC LIMIT 20")
	var recentFiles []File
	for recentRows.Next() {
		var f File
		recentRows.Scan(&f.ID, &f.Filename)
		recentFiles = append(recentFiles, f)
	}
	recentRows.Close()

	return BulkTagFormData{
		Categories:  cats,
		RecentFiles: recentFiles,
		FormData: struct {
			FileRange string
			Category  string
			Value     string
			Operation string
			TagQuery      string
			SelectionMode string
		}{Operation: "add"},
	}
}

func bulkTagHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		formData := getBulkTagFormData()
		pageData := buildPageData("Bulk Tag Editor", formData)
		renderTemplate(w, "bulk-tag.html", pageData)
		return
	}
	if r.Method == http.MethodPost {
		rangeStr := strings.TrimSpace(r.FormValue("file_range"))
		tagQuery := strings.TrimSpace(r.FormValue("tag_query"))
		selectionMode := r.FormValue("selection_mode")
		category := strings.TrimSpace(r.FormValue("category"))
		value := strings.TrimSpace(r.FormValue("value"))
		operation := r.FormValue("operation")

		formData := getBulkTagFormData()
		formData.FormData.FileRange = rangeStr
		formData.FormData.TagQuery = tagQuery
		formData.FormData.SelectionMode = selectionMode
		formData.FormData.Category = category
		formData.FormData.Value = value
		formData.FormData.Operation = operation

		createErrorResponse := func(errorMsg string) {
			formData.Error = errorMsg
			pageData := buildPageData("Bulk Tag Editor", formData)
			renderTemplate(w, "bulk-tag.html", pageData)
		}

		// Validate selection mode
		if selectionMode == "" {
			selectionMode = "range" // default
		}

		// Validate inputs based on selection mode
		if selectionMode == "range" && rangeStr == "" {
			createErrorResponse("File range cannot be empty")
			return
		}
		if selectionMode == "tags" && tagQuery == "" {
			createErrorResponse("Tag query cannot be empty")
			return
		}
		if category == "" {
			createErrorResponse("Category cannot be empty")
			return
		}
		if operation == "add" && value == "" {
			createErrorResponse("Value cannot be empty when adding tags")
			return
		}

		// Get file IDs based on selection mode
		var fileIDs []int
		var err error

		if selectionMode == "range" {
			fileIDs, err = parseFileIDRange(rangeStr)
			if err != nil {
				createErrorResponse(fmt.Sprintf("Invalid file range: %v", err))
				return
			}
		} else if selectionMode == "tags" {
			fileIDs, err = getFileIDsFromTagQuery(tagQuery)
			if err != nil {
				createErrorResponse(fmt.Sprintf("Tag query error: %v", err))
				return
			}
			if len(fileIDs) == 0 {
				createErrorResponse("No files match the tag query")
				return
			}
		} else {
			createErrorResponse("Invalid selection mode")
			return
		}

		validFiles, err := validateFileIDs(fileIDs)
		if err != nil {
			createErrorResponse(fmt.Sprintf("File validation error: %v", err))
			return
		}

		err = applyBulkTagOperations(fileIDs, category, value, operation)
		if err != nil {
			createErrorResponse(fmt.Sprintf("Tag operation failed: %v", err))
			return
		}

		// Build success message
		var successMsg string
		var selectionDesc string
		if selectionMode == "range" {
			selectionDesc = fmt.Sprintf("file range '%s'", rangeStr)
		} else {
			selectionDesc = fmt.Sprintf("tag query '%s'", tagQuery)
		}

		if operation == "add" {
			successMsg = fmt.Sprintf("Tag '%s: %s' added to %d files matching %s",
				category, value, len(validFiles), selectionDesc)
		} else {
			if value != "" {
				successMsg = fmt.Sprintf("Tag '%s: %s' removed from %d files matching %s",
					category, value, len(validFiles), selectionDesc)
			} else {
				successMsg = fmt.Sprintf("All '%s' category tags removed from %d files matching %s",
					category, len(validFiles), selectionDesc)
			}
		}

		// Add file list
		var filenames []string
		for _, f := range validFiles {
			filenames = append(filenames, f.Filename)
		}
		if len(filenames) <= 5 {
			successMsg += fmt.Sprintf(": %s", strings.Join(filenames, ", "))
		} else {
			successMsg += fmt.Sprintf(": %s and %d more", strings.Join(filenames[:5], ", "), len(filenames)-5)
		}

		formData.Success = successMsg
		pageData := buildPageData("Bulk Tag Editor", formData)
		renderTemplate(w, "bulk-tag.html", pageData)
		return
	}
	renderError(w, "Method not allowed", http.StatusMethodNotAllowed)
}


func parseFileIDRange(rangeStr string) ([]int, error) {
	var fileIDs []int
	parts := strings.Split(rangeStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start ID in range %s: %v", part, err)
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end ID in range %s: %v", part, err)
			}

			if start > end {
				return nil, fmt.Errorf("invalid range %s: start must be <= end", part)
			}

			for i := start; i <= end; i++ {
				fileIDs = append(fileIDs, i)
			}
		} else {
			id, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid file ID: %s", part)
			}
			fileIDs = append(fileIDs, id)
		}
	}

	uniqueIDs := make(map[int]bool)
	var result []int
	for _, id := range fileIDs {
		if !uniqueIDs[id] {
			uniqueIDs[id] = true
			result = append(result, id)
		}
	}

	return result, nil
}

func getFileIDsFromTagQuery(query string) ([]int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Check if query contains OR operator
	if strings.Contains(strings.ToUpper(query), " OR ") {
		return getFileIDsFromORQuery(query)
	}

	// Otherwise treat as AND query (comma-separated or single tag)
	return getFileIDsFromANDQuery(query)
}

// getFileIDsFromANDQuery handles comma-separated tags (AND logic)
func getFileIDsFromANDQuery(query string) ([]int, error) {
	tagPairs := strings.Split(query, ",")
	var tags []TagPair

	for _, pair := range tagPairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid tag format '%s', expected 'category:value'", pair)
		}

		tags = append(tags, TagPair{
			Category: strings.TrimSpace(parts[0]),
			Value:    strings.TrimSpace(parts[1]),
		})
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no valid tags found in query")
	}

	// Query database for files matching ALL tags
	return findFilesWithAllTags(tags)
}

// getFileIDsFromORQuery handles OR-separated tags
func getFileIDsFromORQuery(query string) ([]int, error) {
	tagPairs := strings.Split(strings.ToUpper(query), " OR ")
	var tags []TagPair

	for _, pair := range tagPairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid tag format '%s', expected 'category:value'", pair)
		}

		tags = append(tags, TagPair{
			Category: strings.TrimSpace(parts[0]),
			Value:    strings.TrimSpace(parts[1]),
		})
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no valid tags found in query")
	}

	// Query database for files matching ANY tag
	return findFilesWithAnyTag(tags)
}

func validateFileIDs(fileIDs []int) ([]File, error) {
	if len(fileIDs) == 0 {
		return nil, fmt.Errorf("no file IDs provided")
	}

	placeholders := make([]string, len(fileIDs))
	args := make([]interface{}, len(fileIDs))
	for i, id := range fileIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("SELECT id, filename, path FROM files WHERE id IN (%s) ORDER BY id",
		strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer rows.Close()

	var files []File
	foundIDs := make(map[int]bool)

	for rows.Next() {
		var f File
		err := rows.Scan(&f.ID, &f.Filename, &f.Path)
		if err != nil {
			return nil, fmt.Errorf("error scanning file: %v", err)
		}
		files = append(files, f)
		foundIDs[f.ID] = true
	}

	var missingIDs []int
	for _, id := range fileIDs {
		if !foundIDs[id] {
			missingIDs = append(missingIDs, id)
		}
	}

	if len(missingIDs) > 0 {
		return files, fmt.Errorf("file IDs not found: %v", missingIDs)
	}

	return files, nil
}

func findFilesWithAnyTag(tags []TagPair) ([]int, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags specified")
	}

	// Build query with OR conditions
	query := `
		SELECT DISTINCT f.id
		FROM files f
		INNER JOIN file_tags ft ON f.id = ft.file_id
		INNER JOIN tags t ON ft.tag_id = t.id
		INNER JOIN categories c ON t.category_id = c.id
		WHERE `

	var conditions []string
	var args []interface{}
	argIndex := 1

	for _, tag := range tags {
		conditions = append(conditions, fmt.Sprintf(
			"(c.name = $%d AND t.value = $%d)",
			argIndex, argIndex+1))
		args = append(args, tag.Category, tag.Value)
		argIndex += 2
	}

	query += strings.Join(conditions, " OR ")
	query += " ORDER BY f.id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var fileIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}

	return fileIDs, rows.Err()
}


func findFilesWithAllTags(tags []TagPair) ([]int, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags specified")
	}

	// Build query with subqueries for each tag
	query := `
		SELECT f.id
		FROM files f
		WHERE `

	var conditions []string
	var args []interface{}
	argIndex := 1

	for _, tag := range tags {
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM file_tags ft
				JOIN tags t ON ft.tag_id = t.id
				JOIN categories c ON t.category_id = c.id
				WHERE ft.file_id = f.id
				AND c.name = $%d
				AND t.value = $%d
			)`, argIndex, argIndex+1))
		args = append(args, tag.Category, tag.Value)
		argIndex += 2
	}

	query += strings.Join(conditions, " AND ")
	query += " ORDER BY f.id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var fileIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}

	return fileIDs, rows.Err()
}