package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	perPage := 100
	if config.ItemsPerPage != "" {
		if n, err := strconv.Atoi(config.ItemsPerPage); err == nil && n > 0 {
			perPage = n
		}
	}

	var files []File
	var total int
	var searchTitle string

	if query != "" {
		var err error
		files, total, err = getSearchResultsPaginated(query, page, perPage)
		if err != nil {
			renderError(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		searchTitle = fmt.Sprintf("Search Results for: %s", query)
	} else {
		searchTitle = "Search Files"
	}

	pageData := buildPageDataWithPagination(searchTitle, files, page, total, perPage, r)
	pageData.Query = query
	pageData.Files = files
	renderTemplate(w, "search.html", pageData)
}