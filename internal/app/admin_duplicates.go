package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Binary name
var CzkawkaBin = "czkawka_cli"
// dupEntryRe matches lines like: "/path/to/img.jpg" - 1280x1280 - 124.57 KiB - Original
var dupEntryRe = regexp.MustCompile(`^"(.*)" - (\d+x\d+) - (.+?) - ([^-]+)$`)

type DupItem struct {
	Path       string `json:"path"`
	Dims       string `json:"dims"`
	Size       string `json:"size"`
	Similarity string `json:"similarity"`
}

type dupState struct {
	mu      sync.Mutex
	dirs    []string
	groups  [][]DupItem
	allowed map[string]bool
}
var dups = &dupState{allowed: map[string]bool{}}

// Only allow one czkawka scan at a time.
var scanMu sync.Mutex

// read report text output and parse into groups
func parseDuplicateResults(file string) ([][]DupItem, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var groups [][]DupItem
	var cur []DupItem
	flush := func() {
		if len(cur) >= 2 {
			groups = append(groups, cur)
		}
		cur = nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if m := dupEntryRe.FindStringSubmatch(line); m != nil {
			cur = append(cur, DupItem{
				Path:       m[1],
				Dims:       m[2],
				Size:       m[3],
				Similarity: strings.TrimSpace(m[4]),
			})
			continue
		}
		// Anything else (blank line, "Found N images...", headers) ends a group.
		flush()
	}
	flush()
	return groups, nil
}

func (s *dupState) set(dirs []string, groups [][]DupItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs = dirs
	s.groups = groups
	s.allowed = map[string]bool{}
	for _, g := range groups {
		for _, it := range g {
			s.allowed[it.Path] = true
		}
	}
}

// liveGroups drops groups missing files and with less than 2 files
func (s *dupState) liveGroups() [][]DupItem {
	out := make([][]DupItem, 0, len(s.groups))
	for _, g := range s.groups {
		var live []DupItem
		for _, it := range g {
			if _, err := os.Stat(it.Path); err == nil {
				live = append(live, it)
			}
		}
		if len(live) >= 2 {
			out = append(out, live)
		}
	}
	return out
}

// sameGroup reports whether a and b appear together in one results group.
func (s *dupState) sameGroup(a, b string) bool {
	for _, g := range s.groups {
		var hasA, hasB bool
		for _, it := range g {
			hasA = hasA || it.Path == a
			hasB = hasB || it.Path == b
		}
		if hasA && hasB {
			return true
		}
	}
	return false
}

// remainingInGroup counts the existing files in the group containing path.
func (s *dupState) remainingInGroup(path string) int {
	for _, g := range s.groups {
		found := false
		for _, it := range g {
			if it.Path == path {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		n := 0
		for _, it := range g {
			if _, err := os.Stat(it.Path); err == nil {
				n++
			}
		}
		return n
	}
	return 0
}

// dupResponse is what the scan and groups endpoints return.
type dupResponse struct {
	Dirs   []string    `json:"dirs"`
	Groups [][]DupItem `json:"groups"`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Warning: writeJSON: %v", err)
	}
}

// check JSON POST for CORS so other sites can't request actions
func decodeJSONPost(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return false
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	return true
}

// cleans submitted directories, checks they exist and  drops blanks and duplicates
func resolveScanDirs(inputs []string) ([]string, error) {
	var dirs []string
	seen := map[string]bool{}
	for _, in := range inputs {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		abs, err := filepath.Abs(in)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", in, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", abs, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", abs)
		}
		if !seen[abs] {
			seen[abs] = true
			dirs = append(dirs, abs)
		}
	}
	if len(dirs) == 0 {
		return nil, errors.New("enter at least one directory to scan")
	}
	return dirs, nil
}

// pathWithin reports whether p is dir itself or located beneath it.
func pathWithin(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// run binary and write report
func runCzkawka(ctx context.Context, resultsFile string, dirs []string) error {
	args := []string{"image"}
	for _, d := range dirs {
		args = append(args, "-d", d)
	}
	// Video thumbnails live in the uploads directory; they are derived files, not duplicates.
	if uploadAbs, err := filepath.Abs(Cfg.UploadDir); err == nil {
		thumbDir := filepath.Join(uploadAbs, "thumbnails")
		for _, d := range dirs {
			if pathWithin(thumbDir, d) {
				if info, err := os.Stat(thumbDir); err == nil && info.IsDir() {
					args = append(args, "-e", thumbDir)
				}
				break
			}
		}
	}
	args = append(args, "-f", resultsFile)

	log.Printf("Info: runCzkawka: %s %s", CzkawkaBin, strings.Join(args, " "))
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, CzkawkaBin, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	// czkawka_cli exits with 11 when it found results; that's not a failure.
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 11 {
		return nil
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("%s was not found; install czkawka_cli or pass its location with the -czkawka flag", CzkawkaBin)
		}
		tail := strings.TrimSpace(out.String())
		if len(tail) > 600 {
			tail = "..." + tail[len(tail)-600:]
		}
		if tail != "" {
			return fmt.Errorf("czkawka failed: %w: %s", err, tail)
		}
		return fmt.Errorf("czkawka failed: %w", err)
	}
	return nil
}

func duplicatesScanHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dirs []string `json:"dirs"`
	}
	if !decodeJSONPost(w, r, &req) {
		return
	}

	dirs, err := resolveScanDirs(req.Dirs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !scanMu.TryLock() {
		http.Error(w, "a scan is already running", http.StatusConflict)
		return
	}
	defer scanMu.Unlock()

	tmpDir, err := os.MkdirTemp("", "tagliatelle-duplicates-")
	if err != nil {
		log.Printf("Error: duplicatesScanHandler: failed to create temp dir: %v", err)
		http.Error(w, "failed to create temporary directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)
	resultsFile := filepath.Join(tmpDir, "duplicates.log")

	if err := runCzkawka(r.Context(), resultsFile, dirs); err != nil {
		log.Printf("Error: duplicatesScanHandler: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	groups, err := parseDuplicateResults(resultsFile)
	if errors.Is(err, os.ErrNotExist) {
		groups, err = nil, nil // czkawka writes no file when nothing was found
	}
	if err != nil {
		log.Printf("Error: duplicatesScanHandler: failed to read results: %v", err)
		http.Error(w, "failed to read czkawka results", http.StatusInternalServerError)
		return
	}
	log.Printf("Info: duplicatesScanHandler: found %d groups in %s", len(groups), strings.Join(dirs, ", "))

	dups.set(dirs, groups)
	dups.mu.Lock()
	defer dups.mu.Unlock()
	writeJSON(w, dupResponse{Dirs: dirs, Groups: dups.liveGroups()})
}

func duplicatesGroupsHandler(w http.ResponseWriter, r *http.Request) {
	dups.mu.Lock()
	defer dups.mu.Unlock()
	dirs := dups.dirs
	if dirs == nil {
		dirs = []string{}
	}
	writeJSON(w, dupResponse{Dirs: dirs, Groups: dups.liveGroups()})
}

func duplicatesImageHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	dups.mu.Lock()
	ok := dups.allowed[p]
	dups.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeFile(w, r, p)
}

func duplicatesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSONPost(w, r, &req) {
		return
	}

	dups.mu.Lock()
	defer dups.mu.Unlock()
	if !dups.allowed[req.Path] {
		http.Error(w, "path not in results", http.StatusForbidden)
		return
	}
	if dups.remainingInGroup(req.Path) < 2 {
		http.Error(w, "refusing to delete the last remaining file in a group", http.StatusConflict)
		return
	}
	if err := os.Remove(req.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("Error: duplicatesDeleteHandler: failed to delete %s: %v", req.Path, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	forgetUploadRecord(req.Path)
	log.Printf("Info: duplicatesDeleteHandler: deleted %s", req.Path)
	w.WriteHeader(http.StatusNoContent)
}

// duplicatesDeleteGroupHandler deletes every listed file of one results group,
func duplicatesDeleteGroupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if !decodeJSONPost(w, r, &req) {
		return
	}
	if len(req.Paths) == 0 {
		http.Error(w, "no paths given", http.StatusBadRequest)
		return
	}

	dups.mu.Lock()
	defer dups.mu.Unlock()
	for _, p := range req.Paths {
		if !dups.allowed[p] || !dups.sameGroup(req.Paths[0], p) {
			http.Error(w, "paths are not in one group of the results", http.StatusForbidden)
			return
		}
	}
	var failed []string
	for _, p := range req.Paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("Error: duplicatesDeleteGroupHandler: failed to delete %s: %v", p, err)
			failed = append(failed, p)
			continue
		}
		forgetUploadRecord(p)
		log.Printf("Info: duplicatesDeleteGroupHandler: deleted %s", p)
	}
	if len(failed) > 0 {
		http.Error(w, "could not delete: "+strings.Join(failed, ", "), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func duplicatesReplaceHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Src string `json:"src"` // the better file; it is moved
		Dst string `json:"dst"` // the file being replaced
	}
	if !decodeJSONPost(w, r, &req) {
		return
	}

	dups.mu.Lock()
	defer dups.mu.Unlock()
	if req.Src == req.Dst {
		http.Error(w, "src and dst are the same file", http.StatusBadRequest)
		return
	}
	if !dups.allowed[req.Src] || !dups.allowed[req.Dst] || !dups.sameGroup(req.Src, req.Dst) {
		http.Error(w, "paths are not a matching pair in the results", http.StatusForbidden)
		return
	}

	dest, err := replaceFile(req.Src, req.Dst)
	if err != nil {
		log.Printf("Error: duplicatesReplaceHandler: %s -> %s: %v", req.Src, req.Dst, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	syncUploadRecordsAfterReplace(req.Src, req.Dst, dest)
	log.Printf("Info: duplicatesReplaceHandler: replaced %s with %s (now at %s)", req.Dst, req.Src, dest)
	writeJSON(w, map[string]string{"path": dest})
}

// moves src into dst's location
// On extension mismatch, keep src's extension and remove dst's
func replaceFile(src, dst string) (string, error) {
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(src); err != nil {
		return "", err
	}

	dest := dst
	if sx, dx := filepath.Ext(src), filepath.Ext(dst); !strings.EqualFold(sx, dx) {
		dest = strings.TrimSuffix(dst, dx) + sx
		if _, err := os.Lstat(dest); err == nil {
			return "", fmt.Errorf("%s already exists", dest)
		}
	}

	if err := moveFile(src, dest); err != nil {
		return "", err
	}
	// try to keep existing permissions
	_ = os.Chmod(dest, dstInfo.Mode().Perm())

	if dest != dst {
		if err := os.Remove(dst); err != nil {
			return dest, fmt.Errorf("moved to %s, but could not remove old file: %w", dest, err)
		}
	}
	return dest, nil
}

// rename src to dest. If unable, copy to temp and rename in place then remove src
func moveFile(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".replace-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("copied to %s, but could not remove source: %w", dest, err)
	}
	return nil
}

// try to keep the tagliatelle database in sync
func uploadRecord(absPath string) (id int64, filename, relPath string, ok bool) {
	uploadAbs, err := filepath.Abs(Cfg.UploadDir)
	if err != nil || !pathWithin(absPath, uploadAbs) {
		return 0, "", "", false
	}
	rel, err := filepath.Rel(uploadAbs, absPath)
	if err != nil {
		return 0, "", "", false
	}
	if err := DB.QueryRow("SELECT id, filename FROM files WHERE path = ?", rel).Scan(&id, &filename); err != nil {
		return 0, "", "", false
	}
	return id, filename, rel, true
}

func removeThumbnail(filename string) {
	thumb := filepath.Join(Cfg.UploadDir, "thumbnails", filename+".jpg")
	if err := os.Remove(thumb); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("Warning: removeThumbnail: %s: %v", thumb, err)
	}
}

// remove DB records for removed files in upload dir
func forgetUploadRecord(absPath string) {
	id, filename, _, ok := uploadRecord(absPath)
	if !ok {
		return
	}
	tx, err := DB.Begin()
	if err != nil {
		log.Printf("Warning: forgetUploadRecord: failed to start transaction for %s: %v", absPath, err)
		return
	}
	defer tx.Rollback()
	for _, q := range []string{
		"DELETE FROM file_tags WHERE file_id = ?",
		"DELETE FROM file_properties WHERE file_id = ?",
		"DELETE FROM files WHERE id = ?",
	} {
		if _, err := tx.Exec(q, id); err != nil {
			log.Printf("Warning: forgetUploadRecord: %s (id=%d): %v", q, id, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("Warning: forgetUploadRecord: commit failed for id=%d: %v", id, err)
		return
	}
	removeThumbnail(filename)
}

func syncUploadRecordsAfterReplace(src, dst, dest string) {
	forgetUploadRecord(src)

	id, filename, _, ok := uploadRecord(dst)
	if !ok {
		return
	}

	if dest != dst {
		uploadAbs, err := filepath.Abs(Cfg.UploadDir)
		if err == nil {
			if newRel, err := filepath.Rel(uploadAbs, dest); err == nil {
				newName := filepath.Base(dest)
				removeThumbnail(filename)
				if _, err := DB.Exec("UPDATE files SET filename = ?, path = ? WHERE id = ?", newName, newRel, id); err != nil {
					log.Printf("Warning: syncUploadRecordsAfterReplace: failed to update record id=%d: %v", id, err)
				}
			}
		}
	} else {
		removeThumbnail(filename) // its content changed, so the old thumbnail is stale
	}

	if _, err := DB.Exec("DELETE FROM file_properties WHERE file_id = ?", id); err != nil {
		log.Printf("Warning: syncUploadRecordsAfterReplace: failed to clear properties for id=%d: %v", id, err)
	}
	computeProperties(id, dest)
}
