package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func getTaggedFiles() ([]File, error) {
	return queryFilesWithTags(`
		SELECT DISTINCT f.id, f.filename, f.path, COALESCE(f.description, '') as description
		FROM files f
		JOIN file_tags ft ON ft.file_id = f.id
		ORDER BY f.id DESC
	`)
}

func getUntaggedFiles() ([]File, error) {
	return queryFilesWithTags(`
		SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description
		FROM files f
		LEFT JOIN file_tags ft ON ft.file_id = f.id
		WHERE ft.file_id IS NULL
		ORDER BY f.id DESC
	`)
}

func untaggedFilesHandler(w http.ResponseWriter, r *http.Request) {
	// Get page number from query params
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get per page from config
	perPage := 50
	if config.ItemsPerPage != "" {
		if pp, err := strconv.Atoi(config.ItemsPerPage); err == nil && pp > 0 {
			perPage = pp
		}
	}

	files, total, _ := getUntaggedFilesPaginated(page, perPage)
	pageData := buildPageDataWithPagination("Untagged Files", files, page, total, perPage)
	renderTemplate(w, "untagged.html", pageData)
}

func tagFilterHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	perPage := 50
	if config.ItemsPerPage != "" {
		if pp, err := strconv.Atoi(config.ItemsPerPage); err == nil && pp > 0 {
			perPage = pp
		}
	}

	fullPath := strings.TrimPrefix(r.URL.Path, "/tag/")
	tagPairs := strings.Split(fullPath, "/and/tag/")

	breadcrumbs := []Breadcrumb{
		{Name: "Home", URL: "/"},
		{Name: "Tags", URL: "/tags"},
	}

	var filters []filter
	currentPath := "/tag"

	for i, pair := range tagPairs {
		parts := strings.Split(pair, "/")
		if len(parts) != 2 {
			renderError(w, "Invalid tag filter path", http.StatusBadRequest)
			return
		}

		f := filter{
			Category:   parts[0],
			Value:      parts[1],
			IsPreviews: parts[1] == "previews",
		}

		// Expand with aliases (unless it's a special tag)
		if parts[1] != "unassigned" && parts[1] != "previews" {
			f.Values = expandTagWithAliases(parts[0], parts[1])
		}

		filters = append(filters, f)

		// Build breadcrumb path incrementally
		if i == 0 {
			currentPath += "/" + parts[0] + "/" + parts[1]
		} else {
			currentPath += "/and/tag/" + parts[0] + "/" + parts[1]
		}

		// Add category breadcrumb (only if it's the first occurrence)
		categoryExists := false
		for _, bc := range breadcrumbs {
			if bc.Name == parts[0] {
				categoryExists = true
				break
			}
		}
		if !categoryExists {
			breadcrumbs = append(breadcrumbs, Breadcrumb{
				Name: strings.Title(parts[0]),
				URL:  "/tags#tag-" + parts[0],
			})
		}

		// Add value breadcrumb
		breadcrumbs = append(breadcrumbs, Breadcrumb{
			Name: strings.Title(parts[1]),
			URL:  currentPath,
		})
	}

	// Check if we're in preview mode for any filter
	hasPreviewFilter := false
	for _, f := range filters {
		if f.IsPreviews {
			hasPreviewFilter = true
			break
		}
	}

	if hasPreviewFilter {
		// Handle preview mode
		files, err := getPreviewFiles(filters)
		if err != nil {
			renderError(w, "Failed to fetch preview files", http.StatusInternalServerError)
			return
		}

		var titleParts []string
		for _, f := range filters {
			titleParts = append(titleParts, fmt.Sprintf("%s: %s", f.Category, f.Value))
		}
		title := "Tagged: " + strings.Join(titleParts, " + ")

		pageData := buildPageDataWithPagination(title, ListData{
			Tagged:      files,
			Untagged:    nil,
			Breadcrumbs: []Breadcrumb{},
		}, 1, len(files), len(files))
		pageData.Breadcrumbs = breadcrumbs

		renderTemplate(w, "list.html", pageData)
		return
	}

	// Build count query (existing logic)
	countQuery := `SELECT COUNT(DISTINCT f.id) FROM files f WHERE 1=1`
	countArgs := []interface{}{}

	for _, f := range filters {
		if f.Value == "unassigned" {
			countQuery += `
				AND NOT EXISTS (
					SELECT 1
					FROM file_tags ft
					JOIN tags t ON ft.tag_id = t.id
					JOIN categories c ON c.id = t.category_id
					WHERE ft.file_id = f.id AND c.name = ?
				)`
			countArgs = append(countArgs, f.Category)
		} else {
			// Build OR clause for aliases
			placeholders := make([]string, len(f.Values))
			for i := range f.Values {
				placeholders[i] = "?"
			}

			countQuery += fmt.Sprintf(`
				AND EXISTS (
					SELECT 1
					FROM file_tags ft
					JOIN tags t ON ft.tag_id = t.id
					JOIN categories c ON c.id = t.category_id
					WHERE ft.file_id = f.id AND c.name = ? AND t.value IN (%s)
				)`, strings.Join(placeholders, ","))

			countArgs = append(countArgs, f.Category)
			for _, v := range f.Values {
				countArgs = append(countArgs, v)
			}
		}
	}

	var total int
	err := db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		renderError(w, "Failed to count files", http.StatusInternalServerError)
		return
	}

	// Build main query with pagination (existing logic)
	query := `SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description FROM files f WHERE 1=1`
	args := []interface{}{}

	for _, f := range filters {
		if f.Value == "unassigned" {
			query += `
				AND NOT EXISTS (
					SELECT 1
					FROM file_tags ft
					JOIN tags t ON ft.tag_id = t.id
					JOIN categories c ON c.id = t.category_id
					WHERE ft.file_id = f.id AND c.name = ?
				)`
			args = append(args, f.Category)
		} else {
			// Build OR clause for aliases
			placeholders := make([]string, len(f.Values))
			for i := range f.Values {
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

			args = append(args, f.Category)
			for _, v := range f.Values {
				args = append(args, v)
			}
		}
	}

	offset := (page - 1) * perPage
	query += ` ORDER BY f.id DESC LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	files, err := queryFilesWithTags(query, args...)
	if err != nil {
		renderError(w, "Failed to fetch files", http.StatusInternalServerError)
		return
	}

	var titleParts []string
	for _, f := range filters {
		titleParts = append(titleParts, fmt.Sprintf("%s: %s", f.Category, f.Value))
	}
	title := "Tagged: " + strings.Join(titleParts, ", ")

	pageData := buildPageDataWithPagination(title, ListData{
		Tagged:      files,
		Untagged:    nil,
		Breadcrumbs: []Breadcrumb{},
	}, page, total, perPage)
	pageData.Breadcrumbs = breadcrumbs

	renderTemplate(w, "list.html", pageData)
}

func expandTagWithAliases(category, value string) []string {
	values := []string{value}

	for _, group := range config.TagAliases {
		if group.Category != category {
			continue
		}

		// Check if the value is in this alias group
		found := false
		for _, alias := range group.Aliases {
			if strings.EqualFold(alias, value) {
				found = true
				break
			}
		}

		if found {
			// Add all aliases from this group
			for _, alias := range group.Aliases {
				if !strings.EqualFold(alias, value) {
					values = append(values, alias)
				}
			}
			break
		}
	}

	return values
}

func queryFilesWithTags(query string, args ...interface{}) ([]File, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Filename, &f.Path, &f.Description); err != nil {
			return nil, err
		}
		f.EscapedFilename = url.PathEscape(f.Filename)
		files = append(files, f)
	}
	return files, nil
}