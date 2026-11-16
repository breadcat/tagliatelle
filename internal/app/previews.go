package app

import (
	"fmt"
)

// getPreviewFiles returns one representative file for each tag value in the specified category
func getPreviewFiles(filters []filter) ([]File, error) {
	var previewCategory string
	var otherFilters []filter
	for _, f := range filters {
		if f.IsPreviews {
			if previewCategory == "" {
				previewCategory = f.Category
			}
			continue
		}
		otherFilters = append(otherFilters, f)
	}

	if previewCategory == "" {
		return []File{}, nil
	}

	// Discover only the tag values in previewCategory that actually co-occur
	// with the other filters, instead of every value that exists anywhere.
	otherWhere, otherArgs := buildTagFilterWhere(otherFilters)

	tagQuery := `
		SELECT DISTINCT t.value
		FROM tags t
		JOIN categories c ON t.category_id = c.id
		JOIN file_tags ft ON ft.tag_id = t.id
		JOIN files f ON f.id = ft.file_id` +
		otherWhere + ` AND c.name = ?
		ORDER BY t.value`

	tagArgs := append(append([]interface{}(nil), otherArgs...), previewCategory)

	tagRows, err := DB.Query(tagQuery, tagArgs...)
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
	if err := tagRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tag values: %w", err)
	}

	if len(tagValues) == 0 {
		return []File{}, nil
	}

	// For each candidate tag value, find one representative file matching
	// that value plus all the other active filters.
	var allFiles []File
	for _, tagValue := range tagValues {
		valueFilters := make([]filter, 0, len(otherFilters)+1)
		valueFilters = append(valueFilters, otherFilters...)
		valueFilters = append(valueFilters, filter{
			Category: previewCategory,
			Value:    tagValue,
			Values:   []string{tagValue},
		})

		where, args := buildTagFilterWhere(valueFilters)

		query := `SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description
			FROM files f` + where + ` ORDER BY f.id DESC LIMIT 1`

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
