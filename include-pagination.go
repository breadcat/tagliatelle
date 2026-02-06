package main

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

func buildPageDataWithPagination(title string, data interface{}, page, total, perPage int) PageData {
	pd := buildPageData(title, data)
	pd.Pagination = calculatePagination(page, total, perPage)
	return pd
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