package main

type File struct {
	ID              int
	Filename        string
	EscapedFilename string
	Path            string
	Description     string
	Tags            map[string][]string
}

type Config struct {
	// Values from CLI arguments
	DatabasePath string
	UploadDir    string
	ServerPort   string
	// Values from database
	GallerySize  string
	ItemsPerPage string
	TagAliases   []TagAliasGroup
	SedRules     []SedRule
}

type Breadcrumb struct {
	Name string
	URL  string
}

type TagAliasGroup struct {
	Category string   `json:"category"`
	Aliases  []string `json:"aliases"`
}

type TagDisplay struct {
	Value string
	Count int
}

type PropertyDisplay struct {
	Value string
	Count int
}

type ListData struct {
	Tagged      []File
	Untagged    []File
	Breadcrumbs []Breadcrumb
}

type PageData struct {
	Title       string
	Data        interface{}
	Query       string
	IP          string
	Port        string
	Files       []File
	Tags        map[string][]TagDisplay
	Properties  map[string][]PropertyDisplay
	Breadcrumbs []Breadcrumb
	Pagination  *Pagination
	GallerySize string
}

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

type VideoFile struct {
	ID              int
	Filename        string
	Path            string
	HasThumbnail    bool
	ThumbnailPath   string
	EscapedFilename string
}

type filter struct {
	Category   string
	Value      string
	Values     []string // Expanded values including aliases
	IsPreviews bool     // New field to indicate preview mode
	IsProperty bool
}

type BulkTagFormData struct {
	Categories  []string
	RecentFiles []File
	Error       string
	Success     string
	FormData    struct {
		FileRange     string
		Category      string
		Value         string
		Operation     string
		TagQuery      string
		SelectionMode string
	}
}

type TagPair struct {
	Category string
	Value    string
}

type OrphanData struct {
	Orphans        []string // on disk, not in DB
	ReverseOrphans []string // in DB, not on disk
}

type Note struct {
	Category string
	Value    string
	Original string // The full line as stored
}

type SedRule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

type CBZImage struct {
	Filename string
	Index    int
}

type AdminPageData struct {
	Config            Config
	Error             string
	Success           string
	OrphanData        OrphanData
	ActiveTab         string
	MissingThumbnails []VideoFile
}

type notesAnalysis struct {
	Stats      map[string]int
	Categories []string
	LineCount  int
}