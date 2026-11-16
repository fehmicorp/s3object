package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// InitSQLite opens (or creates) the SQLite database and ensures a persistent signing secret exists.
func InitSQLite(storagePath string) (*sql.DB, string, error) {
	if err := os.MkdirAll(storagePath, 0o750); err != nil {
		return nil, "", fmt.Errorf("create storage dir: %w", err)
	}

	dbPath := filepath.Join(storagePath, "data.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("configure sqlite pragmas: %w", err)
	}

	// Create settings table for server secrets & metadata
	schema := `
	CREATE TABLE IF NOT EXISTS server_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("create sqlite schema: %w", err)
	}

	// Load existing presign secret or generate a new 32-byte cryptographic secret
	var secret string
	err = db.QueryRow(`SELECT value FROM server_settings WHERE key = 'presign_secret'`).Scan(&secret)
	if err == sql.ErrNoRows {
		b := make([]byte, 32)
		if _, rErr := rand.Read(b); rErr != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("generate secret: %w", rErr)
		}
		secret = hex.EncodeToString(b)
		if _, iErr := db.Exec(`INSERT INTO server_settings (key, value) VALUES ('presign_secret', ?)`, secret); iErr != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("save presign secret: %w", iErr)
		}
	} else if err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("query presign secret: %w", err)
	}

	return db, secret, nil
}
