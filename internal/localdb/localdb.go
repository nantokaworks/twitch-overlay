package localdb

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var DBClient *sql.DB

type Token struct {
	AccessToken  string
	RefreshToken string
	Scope        string
	ExpiresAt    int64
}

func SetupDB(dbPath string) (*sql.DB, error) {
	if DBClient != nil {
		return DBClient, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	DBClient = db

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tokens (
		id INTEGER PRIMARY KEY,
		access_token TEXT,
		refresh_token TEXT,
		scope TEXT,
		expires_at INTEGER
	)`)
	if err != nil {
		return nil, err
	}

	return db, nil
}
