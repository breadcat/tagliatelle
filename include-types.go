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
	DatabasePath string          `json:"database_path"`
	UploadDir    string          `json:"upload_dir"`
	ServerPort   string          `json:"server_port"`
	InstanceName string          `json:"instance_name"`
	GallerySize  string          `json:"gallery_size"`
	ItemsPerPage string          `json:"items_per_page"`
	TagAliases   []TagAliasGroup `json:"tag_aliases"`
	SedRules     []SedRule       `json:"sed_rules"`
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

type ListData struct {
    Tagged      []File
    Untagged    []File
    Breadcrumbs []Breadcrumb
}

type PageData struct {
	Title      string
	Data       interface{}
	Query      string
	IP         string
	Port       string
	Files      []File
	Tags       map[string][]TagDisplay
	Breadcrumbs []Breadcrumb
	Pagination *Pagination
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
}

type BulkTagFormData struct {
	Categories  []string
	RecentFiles []File
	Error       string
	Success     string
	FormData    struct {
		FileRange string
		Category  string
		Value     string
		Operation string
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

type Operation struct {
	Name        string
	Description string
	Type        string // "sed", "regex", "builtin"
	Command     string
}

type SedRule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
}
