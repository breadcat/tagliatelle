package main

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func getPreviousTagValue(category string, excludeFileID int) (string, error) {
	var value string
	err := db.QueryRow(`
		SELECT t.value
		FROM tags t
		JOIN categories c ON c.id = t.category_id
		JOIN file_tags ft ON ft.tag_id = t.id
		JOIN files f ON f.id = ft.file_id
		WHERE c.name = ? AND ft.file_id != ?
		ORDER BY ft.rowid DESC
		LIMIT 1
	`, category, excludeFileID).Scan(&value)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no previous tag found for category: %s", category)
	}
	if err != nil {
		return "", err
	}

	return value, nil
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/file/")
	if strings.Contains(idStr, "/") {
		idStr = strings.SplitN(idStr, "/", 2)[0]
	}

	var f File
	err := db.QueryRow("SELECT id, filename, path, COALESCE(description, '') as description FROM files WHERE id=?", idStr).Scan(&f.ID, &f.Filename, &f.Path, &f.Description)
	if err != nil {
		renderError(w, "File not found", http.StatusNotFound)
		return
	}

	f.Tags = make(map[string][]string)
	rows, _ := db.Query(`
		SELECT c.name, t.value
		FROM tags t
		JOIN categories c ON c.id = t.category_id
		JOIN file_tags ft ON ft.tag_id = t.id
		WHERE ft.file_id=?`, f.ID)
	for rows.Next() {
		var cat, val string
		rows.Scan(&cat, &val)
		f.Tags[cat] = append(f.Tags[cat], val)
	}
	rows.Close()

	if r.Method == http.MethodPost {
		if r.FormValue("action") == "update_description" {
			description := r.FormValue("description")
			if len(description) > 2048 {
				description = description[:2048]
			}

			if _, err := db.Exec("UPDATE files SET description = ? WHERE id = ?", description, f.ID); err != nil {
				renderError(w, "Failed to update description", http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/file/"+idStr, http.StatusSeeOther)
			return
		}
		cat := strings.TrimSpace(r.FormValue("category"))
		val := strings.TrimSpace(r.FormValue("value"))
		if cat != "" && val != "" {
			originalVal := val
			if val == "!" {
				previousVal, err := getPreviousTagValue(cat, f.ID)
				if err != nil {
					http.Redirect(w, r, "/file/"+idStr+"?error="+url.QueryEscape("No previous tag found for category: "+cat), http.StatusSeeOther)
					return
				}
				val = previousVal
			}
			_, tagID, err := getOrCreateCategoryAndTag(cat, val)
			if err != nil {
				http.Redirect(w, r, "/file/"+idStr+"?error="+url.QueryEscape("Failed to create tag: "+err.Error()), http.StatusSeeOther)
				return
			}
			_, err = db.Exec("INSERT OR IGNORE INTO file_tags(file_id, tag_id) VALUES (?, ?)", f.ID, tagID)
			if err != nil {
				http.Redirect(w, r, "/file/"+idStr+"?error="+url.QueryEscape("Failed to add tag: "+err.Error()), http.StatusSeeOther)
				return
			}
			if originalVal == "!" {
				http.Redirect(w, r, "/file/"+idStr+"?success="+url.QueryEscape("Tag '"+cat+": "+val+"' copied from previous file"), http.StatusSeeOther)
				return
			}
		}
		http.Redirect(w, r, "/file/"+idStr, http.StatusSeeOther)
		return
	}

	catRows, _ := db.Query(`
		SELECT DISTINCT c.name
		FROM categories c
		JOIN tags t ON t.category_id = c.id
		JOIN file_tags ft ON ft.tag_id = t.id
		ORDER BY c.name
	`)
	var cats []string
	for catRows.Next() {
		var c string
		catRows.Scan(&c)
		cats = append(cats, c)
	}
	catRows.Close()

	pageData := buildPageDataWithIP(f.Filename, struct {
		File            File
		Categories      []string
		EscapedFilename string
	}{f, cats, url.PathEscape(f.Filename)})

	renderTemplate(w, "file.html", pageData)
}

func buildPageDataWithIP(title string, data interface{}) PageData {
	pageData := buildPageData(title, data)
	ip, _ := getLocalIP()
	pageData.IP = ip
	pageData.Port = strings.TrimPrefix(config.ServerPort, ":")
	return pageData
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no connected network interface found")
}

func tagActionHandler(w http.ResponseWriter, r *http.Request, parts []string) {
	fileID := parts[2]
	cat := parts[4]
	val := parts[5]
	action := parts[6]

	if action == "delete" && r.Method == http.MethodPost {
		var tagID int
		db.QueryRow(`
			SELECT t.id
			FROM tags t
			JOIN categories c ON c.id=t.category_id
			WHERE c.name=? AND t.value=?`, cat, val).Scan(&tagID)
		if tagID != 0 {
			db.Exec("DELETE FROM file_tags WHERE file_id=? AND tag_id=?", fileID, tagID)
		}
	}
	http.Redirect(w, r, "/file/"+fileID, http.StatusSeeOther)
}

func getOrCreateCategoryAndTag(category, value string) (int, int, error) {
	category = strings.TrimSpace(category)
	value = strings.TrimSpace(value)
	var catID int
	err := db.QueryRow("SELECT id FROM categories WHERE name=?", category).Scan(&catID)
	if err == sql.ErrNoRows {
		res, err := db.Exec("INSERT INTO categories(name) VALUES(?)", category)
		if err != nil {
			return 0, 0, err
		}
		cid, _ := res.LastInsertId()
		catID = int(cid)
	} else if err != nil {
		return 0, 0, err
	}

	var tagID int
	if value != "" {
		err = db.QueryRow("SELECT id FROM tags WHERE category_id=? AND value=?", catID, value).Scan(&tagID)
		if err == sql.ErrNoRows {
			res, err := db.Exec("INSERT INTO tags(category_id, value) VALUES(?, ?)", catID, value)
			if err != nil {
				return 0, 0, err
			}
			tid, _ := res.LastInsertId()
			tagID = int(tid)
		} else if err != nil {
			return 0, 0, err
		}
	}

	return catID, tagID, nil
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
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

	tagged, taggedTotal, _ := getTaggedFilesPaginated(page, perPage)
	untagged, untaggedTotal, _ := getUntaggedFilesPaginated(page, perPage)

	// Use the larger total for pagination
	total := taggedTotal
	if untaggedTotal > total {
		total = untaggedTotal
	}

	pageData := buildPageDataWithPagination("File Browser", ListData{
		Tagged:      tagged,
		Untagged:    untagged,
		Breadcrumbs: []Breadcrumb{},
	}, page, total, perPage, r)

	renderTemplate(w, "list.html", pageData)
}