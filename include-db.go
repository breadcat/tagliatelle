package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// InitDatabase opens the database connection and creates tables if needed
func InitDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
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
	`

	_, err := db.Exec(schema)
	return err
}
