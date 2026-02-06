package main

import (
    "database/sql"
    "fmt"
    "net/http"
    "net/url"
    "strings"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	var files []File
	var searchTitle string

	if query != "" {
		sqlPattern := "%" + strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(query), "*", "%"), "?", "_") + "%"

		rows, err := db.Query(`
			SELECT f.id, f.filename, f.path, COALESCE(f.description, '') AS description,
			       c.name AS category, t.value AS tag
			FROM files f
			LEFT JOIN file_tags ft ON ft.file_id = f.id
			LEFT JOIN tags t ON t.id = ft.tag_id
			LEFT JOIN categories c ON c.id = t.category_id
			WHERE LOWER(f.filename) LIKE ? OR LOWER(f.description) LIKE ? OR LOWER(t.value) LIKE ?
			ORDER BY f.filename
		`, sqlPattern, sqlPattern, sqlPattern)
		if err != nil {
			renderError(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		fileMap := make(map[int]*File)
		for rows.Next() {
			var id int
			var filename, path, description, category, tag sql.NullString

			if err := rows.Scan(&id, &filename, &path, &description, &category, &tag); err != nil {
				renderError(w, "Failed to read search results: "+err.Error(), http.StatusInternalServerError)
				return
			}

			f, exists := fileMap[id]
			if !exists {
				f = &File{
					ID:              id,
					Filename:        filename.String,
					Path:            path.String,
					EscapedFilename: url.PathEscape(filename.String),
					Description:     description.String,
					Tags:            make(map[string][]string),
				}
				fileMap[id] = f
			}

			if category.Valid && tag.Valid && tag.String != "" {
				f.Tags[category.String] = append(f.Tags[category.String], tag.String)
			}
		}

		for _, f := range fileMap {
			files = append(files, *f)
		}

		searchTitle = fmt.Sprintf("Search Results for: %s", query)
	} else {
		searchTitle = "Search Files"
	}

	pageData := buildPageData(searchTitle, files)
	pageData.Query = query
	pageData.Files = files
	renderTemplate(w, "search.html", pageData)
}