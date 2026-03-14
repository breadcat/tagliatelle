package main

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// InitDatabase opens the database connection and creates tables if needed
func InitDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// createTables creates all necessary database tables
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT,
		path TEXT,
		description TEXT DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE
	);
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER,
		value TEXT,
		UNIQUE(category_id, value)
	);
	CREATE TABLE IF NOT EXISTS file_tags (
		file_id INTEGER,
		tag_id INTEGER,
		UNIQUE(file_id, tag_id)
	);
	CREATE TABLE IF NOT EXISTS file_properties (
		file_id INTEGER,
		key   TEXT,
		value TEXT,
		UNIQUE(file_id, key)
	);
	CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		content TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS tag_aliases (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		category TEXT NOT NULL,
		aliases  TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sed_rules (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		command     TEXT NOT NULL
	);
	`

	_, err := db.Exec(schema)
	return err
}

func LoadConfig(db *sql.DB) (Config, error) {
	cfg := Config{
		GallerySize:  "400px",
		ItemsPerPage: "100",
		TagAliases:   []TagAliasGroup{},
		SedRules:     []SedRule{},
	}

	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return cfg, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return cfg, err
		}
		switch key {
		case "gallery_size":
			if value != "" {
				cfg.GallerySize = value
			}
		case "items_per_page":
			if value != "" {
				cfg.ItemsPerPage = value
			}
		}
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}

	aliasRows, err := db.Query(`SELECT category, aliases FROM tag_aliases ORDER BY id`)
	if err != nil {
		return cfg, err
	}
	defer aliasRows.Close()
	for aliasRows.Next() {
		var category, aliasesStr string
		if err := aliasRows.Scan(&category, &aliasesStr); err != nil {
			return cfg, err
		}
		cfg.TagAliases = append(cfg.TagAliases, TagAliasGroup{
			Category: category,
			Aliases:  strings.Split(aliasesStr, "|"),
		})
	}
	if err := aliasRows.Err(); err != nil {
		return cfg, err
	}

	sedRows, err := db.Query(`SELECT name, description, command FROM sed_rules ORDER BY id`)
	if err != nil {
		return cfg, err
	}
	defer sedRows.Close()
	for sedRows.Next() {
		var rule SedRule
		if err := sedRows.Scan(&rule.Name, &rule.Description, &rule.Command); err != nil {
			return cfg, err
		}
		cfg.SedRules = append(cfg.SedRules, rule)
	}
	if err := sedRows.Err(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func SaveConfig(db *sql.DB, cfg Config) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, kv := range [][2]string{
		{"gallery_size", cfg.GallerySize},
		{"items_per_page", cfg.ItemsPerPage},
	} {
		if _, err := tx.Exec(`
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, kv[0], kv[1]); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM tag_aliases`); err != nil {
		return err
	}
	for _, group := range cfg.TagAliases {
		aliasesStr := strings.Join(group.Aliases, "|")
		if _, err := tx.Exec(
			`INSERT INTO tag_aliases (category, aliases) VALUES (?, ?)`,
			group.Category, aliasesStr,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM sed_rules`); err != nil {
		return err
	}
	for _, rule := range cfg.SedRules {
		if _, err := tx.Exec(
			`INSERT INTO sed_rules (name, description, command) VALUES (?, ?, ?)`,
			rule.Name, rule.Description, rule.Command,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
