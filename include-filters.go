package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func untaggedFilesHandler(w http.ResponseWriter, r *http.Request) {
	page := pageFromRequest(r)
	perPage := perPageFromConfig(50)

	files, total, err := getUntaggedFilesPaginated(page, perPage)
	if err != nil {
		log.Printf("Error: untaggedFilesHandler: failed to get untagged files: %v", err)
	}
	pageData := buildPageDataWithPagination("Untagged Files", files, page, total, perPage, r)
	renderTemplate(w, "untagged.html", pageData)
}

func buildTagFilterWhere(filters []filter) (string, []interface{}) {
	where := " WHERE 1=1"
	var args []interface{}

	for _, f := range filters {
		if f.IsProperty {
			where += `
				AND EXISTS (
					SELECT 1
					FROM file_properties fp
					WHERE fp.file_id = f.id AND fp.key = ? AND fp.value = ?
				)`
			args = append(args, f.Category, f.Value)
		} else if f.Value == "unassigned" {
			where += `
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

			where += fmt.Sprintf(`
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

	return where, args
}

// parseFilterSegments splits filter path into individual filter structs
func parseFilterSegments(fullPath, firstKind string) ([]filter, []Breadcrumb, error) {
	rawSegments := strings.Split(fullPath, "/and/")

	breadcrumbs := []Breadcrumb{
		{Name: "home", URL: "/"},
		{Name: "tags", URL: "/tags"},
	}

	var filters []filter
	currentPath := "/" + firstKind

	for i, seg := range rawSegments {
		var kind, category, value string

		if i == 0 {
			// First segment has no explicit kind prefix — the caller supplies it.
			parts := strings.SplitN(seg, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, nil, fmt.Errorf("invalid %s segment: %q", firstKind, seg)
			}
			kind, category, value = firstKind, parts[0], parts[1]
		} else {
			// Subsequent segments are "kind/category/value".
			parts := strings.SplitN(seg, "/", 3)
			if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
				return nil, nil, fmt.Errorf("invalid filter segment: %q", seg)
			}
			kind, category, value = parts[0], parts[1], parts[2]
			if kind != "tag" && kind != "property" {
				return nil, nil, fmt.Errorf("unknown filter kind %q in segment: %q", kind, seg)
			}
		}

		f := filter{
			Category:   category,
			Value:      value,
			IsProperty: kind == "property",
			IsPreviews: kind == "tag" && value == "previews",
		}

		if kind == "tag" && value != "unassigned" && value != "previews" {
			f.Values = expandTagWithAliases(category, value)
		}

		filters = append(filters, f)

		// Build the cumulative breadcrumb URL for this filter step.
		if i == 0 {
			currentPath += "/" + category + "/" + value
		} else {
			currentPath += "/and/" + kind + "/" + category + "/" + value
		}

		// Add a category/key breadcrumb (deduplicated).
		categoryExists := false
		for _, bc := range breadcrumbs {
			if bc.Name == category {
				categoryExists = true
				break
			}
		}
		if !categoryExists {
			anchorURL := "/tags#tag-" + category
			if kind == "property" {
				anchorURL = "/properties#prop-" + category
			}
			breadcrumbs = append(breadcrumbs, Breadcrumb{
				Name: category,
				URL:  anchorURL,
			})
		}

		breadcrumbs = append(breadcrumbs, Breadcrumb{
			Name: value,
			URL:  currentPath,
		})
	}

	return filters, breadcrumbs, nil
}

func tagFilterHandler(w http.ResponseWriter, r *http.Request) {
	page := pageFromRequest(r)
	perPage := perPageFromConfig(50)

	fullPath := strings.TrimPrefix(r.URL.Path, "/tag/")

	filters, breadcrumbs, err := parseFilterSegments(fullPath, "tag")
	if err != nil {
		renderError(w, "Invalid filter path", http.StatusBadRequest)
		return
	}

	// Check if we're in preview mode for any filter.
	hasPreviewFilter := false
	for _, f := range filters {
		if f.IsPreviews {
			hasPreviewFilter = true
			break
		}
	}

	if hasPreviewFilter {
		files, err := getPreviewFiles(filters)
		if err != nil {
			log.Printf("Error: tagFilterHandler: failed to fetch preview files: %v", err)
			renderError(w, "Failed to fetch preview files", http.StatusInternalServerError)
			return
		}

		title := "Tagged: " + buildFilterTitle(filters, " + ")
		pageData := buildPageDataWithPagination(title, ListData{
			Tagged:      files,
			Untagged:    nil,
			Breadcrumbs: []Breadcrumb{},
		}, 1, len(files), len(files), r)
		pageData.Breadcrumbs = breadcrumbs

		renderTemplate(w, "list.html", pageData)
		return
	}

	// Build the shared WHERE clause once and reuse it for both queries.
	where, whereArgs := buildTagFilterWhere(filters)

	var total int
	countArgs := append([]interface{}(nil), whereArgs...) // copy; count query does not need pagination args
	err = db.QueryRow(`SELECT COUNT(DISTINCT f.id) FROM files f`+where, countArgs...).Scan(&total)
	if err != nil {
		log.Printf("Error: tagFilterHandler: failed to count files: %v", err)
		renderError(w, "Failed to count files", http.StatusInternalServerError)
		return
	}

	offset := (page - 1) * perPage
	dataArgs := append(append([]interface{}(nil), whereArgs...), perPage, offset)
	files, err := queryFilesWithTags(
		`SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description FROM files f`+
			where+` ORDER BY f.id DESC LIMIT ? OFFSET ?`,
		dataArgs...,
	)
	if err != nil {
		log.Printf("Error: tagFilterHandler: failed to fetch files: %v", err)
		renderError(w, "Failed to fetch files", http.StatusInternalServerError)
		return
	}

	title := "Tagged: " + buildFilterTitle(filters, ", ")
	pageData := buildPageDataWithPagination(title, ListData{
		Tagged:      files,
		Untagged:    nil,
		Breadcrumbs: []Breadcrumb{},
	}, page, total, perPage, r)
	pageData.Breadcrumbs = breadcrumbs

	renderTemplate(w, "list.html", pageData)
}

func buildFilterTitle(filters []filter, sep string) string {
	var parts []string
	for _, f := range filters {
		parts = append(parts, fmt.Sprintf("%s: %s", f.Category, f.Value))
	}
	return strings.Join(parts, sep)
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