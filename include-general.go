package main

import (
    "net/http"
    "net/url"
    "strings"
)

func sanitizeFilename(filename string) string {
	if filename == "" {
		return "file"
	}
	filename = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(filename, "/", "_"), "\\", "_"), "..", "_")
	if filename == "" {
		return "file"
	}
	return filename
}

func renderError(w http.ResponseWriter, message string, statusCode int) {
	http.Error(w, message, statusCode)
}

func renderTemplate(w http.ResponseWriter, tmplName string, data PageData) {
	if err := tmpl.ExecuteTemplate(w, tmplName, data); err != nil {
		renderError(w, "Template rendering failed", http.StatusInternalServerError)
	}
}

func redirectWithWarning(w http.ResponseWriter, r *http.Request, baseURL, warningMsg string) {
	redirectURL := baseURL
	if warningMsg != "" {
		redirectURL += "?warning=" + url.QueryEscape(warningMsg)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func successString(err error, msg string) string {
	if err == nil {
		return msg
	}
	return ""
}

func buildPageData(title string, data interface{}) PageData {
	tagMap, _ := getTagData()
	return PageData{Title: title, Data: data, Tags: tagMap, GallerySize: config.GallerySize,}
}

func getTagData() (map[string][]TagDisplay, error) {
	rows, err := db.Query(`
		SELECT c.name, t.value, COUNT(ft.file_id)
		FROM tags t
		JOIN categories c ON c.id = t.category_id
		LEFT JOIN file_tags ft ON ft.tag_id = t.id
		GROUP BY t.id
		HAVING COUNT(ft.file_id) > 0
		ORDER BY c.name, t.value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tagMap := make(map[string][]TagDisplay)
	for rows.Next() {
		var cat, val string
		var count int
		rows.Scan(&cat, &val, &count)
		tagMap[cat] = append(tagMap[cat], TagDisplay{Value: val, Count: count})
	}
	return tagMap, nil
}

func tagsHandler(w http.ResponseWriter, r *http.Request) {
	pageData := buildPageData("All Tags", nil)
	pageData.Data = pageData.Tags
	renderTemplate(w, "tags.html", pageData)
}