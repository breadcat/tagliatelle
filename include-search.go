package main

import (
	"fmt"
	"net/http"
	"strings"
)

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/search/"))
	page := pageFromRequest(r)
	perPage := perPageFromConfig(50)

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