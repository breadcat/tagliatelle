package main

import (
	"fmt"
	"strings"
)

// getPreviewFiles returns one representative file for each tag value in the specified category
func getPreviewFiles(filters []filter) ([]File, error) {
	// Find the preview filter category
	var previewCategory string
	for _, f := range filters {
		if f.IsPreviews {
			previewCategory = f.Category
			break
		}
	}

	if previewCategory == "" {
		return []File{}, nil
	}

	// First, get all tag values for the preview category that have files
	tagQuery := `
		SELECT DISTINCT t.value
		FROM tags t
		JOIN categories c ON t.category_id = c.id
		JOIN file_tags ft ON ft.tag_id = t.id
		WHERE c.name = ?
		ORDER BY t.value`

	tagRows, err := db.Query(tagQuery, previewCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to query tag values: %w", err)
	}
	defer tagRows.Close()

	var tagValues []string
	for tagRows.Next() {
		var tagValue string
		if err := tagRows.Scan(&tagValue); err != nil {
			return nil, fmt.Errorf("failed to scan tag value: %w", err)
		}
		tagValues = append(tagValues, tagValue)
	}


	if len(tagValues) == 0 {
		return []File{}, nil
	}

	// For each tag value, find one representative file
	var allFiles []File
	for _, tagValue := range tagValues {
		// Build query for this specific tag value with all filters applied
		query := `SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description
			FROM files f
			WHERE 1=1`
		args := []interface{}{}

		// Apply all filters (including the preview category with this specific value)
		for _, filter := range filters {
			if filter.IsPreviews {
				// For the preview filter, use the current tag value we're iterating over
				query += `
					AND EXISTS (
						SELECT 1
						FROM file_tags ft
						JOIN tags t ON ft.tag_id = t.id
						JOIN categories c ON c.id = t.category_id
						WHERE ft.file_id = f.id AND c.name = ? AND t.value = ?
					)`
				args = append(args, filter.Category, tagValue)
			} else if filter.Value == "unassigned" {
				query += `
					AND NOT EXISTS (
						SELECT 1
						FROM file_tags ft
						JOIN tags t ON ft.tag_id = t.id
						JOIN categories c ON c.id = t.category_id
						WHERE ft.file_id = f.id AND c.name = ?
					)`
				args = append(args, filter.Category)
			} else {
				// Normal filter with aliases
				placeholders := make([]string, len(filter.Values))
				for i := range filter.Values {
					placeholders[i] = "?"
				}

				query += fmt.Sprintf(`
					AND EXISTS (
						SELECT 1
						FROM file_tags ft
						JOIN tags t ON ft.tag_id = t.id
						JOIN categories c ON c.id = t.category_id
						WHERE ft.file_id = f.id AND c.name = ? AND t.value IN (%s)
					)`, strings.Join(placeholders, ","))

				args = append(args, filter.Category)
				for _, v := range filter.Values {
					args = append(args, v)
				}
			}
		}

		query += ` ORDER BY f.id DESC LIMIT 1`

		files, err := queryFilesWithTags(query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to query files for tag %s: %w", tagValue, err)
		}

		if len(files) > 0 {
			allFiles = append(allFiles, files[0])
		}
	}

	return allFiles, nil
}