package app

import (
	"database/sql"
	"html/template"
)

// Shared application state used across handlers in this package.
var (
	DB   *sql.DB
	Tmpl *template.Template
	Cfg  Config
)
