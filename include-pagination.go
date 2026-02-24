package main

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func pageFromRequest(r *http.Request) int {
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		return p
	}
	return 1
}

func perPageFromConfig(fallback int) int {
	if n, err := strconv.Atoi(config.ItemsPerPage); err == nil && n > 0 {
		return n
	}
	return fallback
}

func getUntaggedFilesPaginated(page, perPage int) ([]File, int, error) {
	// Get total count
	var total int
	err := db.QueryRow(`SELECT COUNT(*) FROM files f LEFT JOIN file_tags ft ON ft.file_id = f.id WHERE ft.file_id IS NULL`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	files, err := queryFilesWithTags(`
		SELECT f.id, f.filename, f.path, COALESCE(f.description, '') as description
		FROM files f
		LEFT JOIN file_tags ft ON ft.file_id = f.id
		WHERE ft.file_id IS NULL
		ORDER BY f.id DESC
		LIMIT ? OFFSET ?
	`, perPage, offset)

	return files, total, err
}

func buildPageDataWithPagination(title string, data interface{}, page, total, perPage int, r *http.Request) PageData {
	pd := buildPageData(title, data)
	pd.Pagination = calculatePagination(page, total, perPage)
	pd.Pagination.PageBaseURL = pageBaseURL(r)
	return pd
}

// pageBaseURL returns a URL base suitable for appending page=N.
// It preserves all existing query parameters except 'page'.
// e.g. /search?q=cats  →  "?q=cats&"
//      /browse          →  "?"
func pageBaseURL(r *http.Request) string {
	params := r.URL.Query()
	params.Del("page")
	if encoded := params.Encode(); encoded != "" {
		return "?" + encoded + "&"
	}
	return "?"
}

func calculatePagination(page, total, perPage int) *Pagination {
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	return &Pagination{
		CurrentPage: page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		PerPage:     perPage,
	}
}

func getSearchResultsPaginated(query string, page, perPage int) ([]File, int, error) {
	sqlPattern := "%" + strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(query), "*", "%"), "?", "_") + "%"

	var total int
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT f.id)
		FROM files f
		LEFT JOIN file_tags ft ON ft.file_id = f.id
		LEFT JOIN tags t ON t.id = ft.tag_id
		WHERE LOWER(f.filename) LIKE ? OR LOWER(f.description) LIKE ? OR LOWER(t.value) LIKE ?
	`, sqlPattern, sqlPattern, sqlPattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := db.Query(`
		SELECT f.id, f.filename, f.path, COALESCE(f.description, '') AS description,
		       c.name AS category, t.value AS tag
		FROM files f
		LEFT JOIN file_tags ft ON ft.file_id = f.id
		LEFT JOIN tags t ON t.id = ft.tag_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE f.id IN (
			SELECT DISTINCT f2.id
			FROM files f2
			LEFT JOIN file_tags ft2 ON ft2.file_id = f2.id
			LEFT JOIN tags t2 ON t2.id = ft2.tag_id
			WHERE LOWER(f2.filename) LIKE ? OR LOWER(f2.description) LIKE ? OR LOWER(t2.value) LIKE ?
			ORDER BY f2.filename
			LIMIT ? OFFSET ?
		)
		ORDER BY f.filename
	`, sqlPattern, sqlPattern, sqlPattern, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	fileMap := make(map[int]*File)
	var orderedIDs []int
	for rows.Next() {
		var id int
		var filename, path, description, category, tag sql.NullString
		if err := rows.Scan(&id, &filename, &path, &description, &category, &tag); err != nil {
			return nil, 0, err
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
			orderedIDs = append(orderedIDs, id)
		}
		if category.Valid && tag.Valid && tag.String != "" {
			f.Tags[category.String] = append(f.Tags[category.String], tag.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	files := make([]File, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		files = append(files, *fileMap[id])
	}
	return files, total, nil
}

func getTaggedFilesPaginated(page, perPage int) ([]File, int, error) {
	// Get total count
	var total int
	err := db.QueryRow(`SELECT COUNT(DISTINCT f.id) FROM files f JOIN file_tags ft ON ft.file_id = f.id`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	files, err := queryFilesWithTags(`
		SELECT DISTINCT f.id, f.filename, f.path, COALESCE(f.description, '') as description
		FROM files f
		JOIN file_tags ft ON ft.file_id = f.id
		ORDER BY f.id DESC
		LIMIT ? OFFSET ?
	`, perPage, offset)

	return files, total, err
}