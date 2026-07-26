package app

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Pagination struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
	PerPage     int
	PageBaseURL string
}

func pageFromRequest(r *http.Request) int {
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		return p
	}
	return 1
}

func perPageFromConfig(fallback int) int {
	if n, err := strconv.Atoi(Cfg.ItemsPerPage); err == nil && n > 0 {
		return n
	}
	return fallback
}

func getUntaggedFilesPaginated(page, perPage int) ([]File, int, error) {
	// Get total count
	var total int
	err := DB.QueryRow(`SELECT COUNT(*) FROM files f LEFT JOIN file_tags ft ON ft.file_id = f.id WHERE ft.file_id IS NULL`).Scan(&total)
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
	err := DB.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT f.id
			FROM files f
			LEFT JOIN file_tags ft ON ft.file_id = f.id
			LEFT JOIN tags t ON t.id = ft.tag_id
			WHERE LOWER(f.filename) LIKE ?
			   OR LOWER(f.description) LIKE ?
			   OR LOWER(t.value) LIKE ?
			GROUP BY f.id
		) matched
	`, sqlPattern, sqlPattern, sqlPattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := DB.Query(`
		SELECT
			f.id,
			f.filename,
			f.path,
			COALESCE(f.description, '') AS description,
			c.name AS category,
			t.value AS tag
		FROM (
			SELECT f2.id
			FROM files f2
			LEFT JOIN file_tags ft2 ON ft2.file_id = f2.id
			LEFT JOIN tags t2 ON t2.id = ft2.tag_id
			WHERE LOWER(f2.filename) LIKE ?
			   OR LOWER(f2.description) LIKE ?
			   OR LOWER(t2.value) LIKE ?
			GROUP BY f2.id
			ORDER BY f2.id DESC
			LIMIT ? OFFSET ?
		) matched
		JOIN files f
			ON f.id = matched.id
		LEFT JOIN file_tags ft
			ON ft.file_id = f.id
		LEFT JOIN tags t
			ON t.id = ft.tag_id
		LEFT JOIN categories c
			ON c.id = t.category_id
		ORDER BY f.id DESC, c.name, t.value
	`, sqlPattern, sqlPattern, sqlPattern, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	fileMap := make(map[int]*File)
	files := make([]File, 0, perPage)

	for rows.Next() {
		var (
			id                          int
			filename, path, description sql.NullString
			category, tag               sql.NullString
		)

		if err := rows.Scan(&id, &filename, &path, &description, &category, &tag); err != nil {
			return nil, 0, err
		}
		f, exists := fileMap[id]
		if !exists {
			file := File{
				ID:              id,
				Filename:        filename.String,
				Path:            path.String,
				EscapedFilename: url.PathEscape(filename.String),
				Description:     description.String,
				Tags:            make(map[string][]string),
			}
			files = append(files, file)
			f = &files[len(files)-1]
			fileMap[id] = f
		}
		if category.Valid && tag.Valid && tag.String != "" {
			f.Tags[category.String] = append(f.Tags[category.String], tag.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

func getTaggedFilesPaginated(page, perPage int) ([]File, int, error) {
	// Get total count
	var total int
	err := DB.QueryRow(`SELECT COUNT(DISTINCT f.id) FROM files f JOIN file_tags ft ON ft.file_id = f.id`).Scan(&total)
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
