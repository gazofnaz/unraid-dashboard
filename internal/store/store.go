// Package store is the SQLite persistence layer. It stores only preferences,
// overrides, cached icons, discovery decisions and run history — the live
// container inventory is always rebuilt from Docker at startup.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// migrations run in order inside one transaction each; schema_migrations
// records the applied versions.
var migrations = []string{
	`CREATE TABLE settings (
		key TEXT PRIMARY KEY,
		value_json TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE container_overrides (
		container_key TEXT PRIMARY KEY,
		value_json TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE endpoint_decisions (
		container_key TEXT PRIMARY KEY,
		value_json TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE icon_cache (
		cache_key TEXT PRIMARY KEY,
		mime TEXT,
		body BLOB,
		etag TEXT,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE discovery_runs (
		id INTEGER PRIMARY KEY,
		started_at INTEGER,
		finished_at INTEGER,
		stats_json TEXT
	);`,
}

// Open creates/opens the database at path and applies migrations.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writes are serialized anyway
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	var current int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}
	for i := current; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			i+1, time.Now().Unix()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Settings loads the global settings; found is false on a fresh database.
func (s *Store) Settings() (model.Settings, bool, error) {
	settings := model.DefaultSettings()
	var raw string
	err := s.db.QueryRow(`SELECT value_json FROM settings WHERE key = 'global'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, false, nil
	}
	if err != nil {
		return settings, false, err
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return model.DefaultSettings(), false, nil
	}
	return settings, true, nil
}

// SaveSettings persists the global settings.
func (s *Store) SaveSettings(settings model.Settings) error {
	return s.upsertJSON(`INSERT INTO settings (key, value_json, updated_at) VALUES ('global', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`, settings)
}

// Linkstack loads the launcher configuration. A fresh database, or one whose
// stored value no longer parses, returns the default launcher.
func (s *Store) Linkstack() (model.Linkstack, error) {
	stack := model.DefaultLinkstack()
	var raw string
	err := s.db.QueryRow(`SELECT value_json FROM settings WHERE key = 'linkstack'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return stack, nil
	}
	if err != nil {
		return stack, err
	}
	if err := json.Unmarshal([]byte(raw), &stack); err != nil {
		return model.DefaultLinkstack(), nil
	}
	return stack.Normalize(), nil
}

// SaveLinkstack persists the launcher configuration.
func (s *Store) SaveLinkstack(l model.Linkstack) error {
	return s.upsertJSON(`INSERT INTO settings (key, value_json, updated_at) VALUES ('linkstack', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`, l)
}

// Overrides returns all container overrides keyed by container key.
func (s *Store) Overrides() (map[string]model.Override, error) {
	rows, err := s.db.Query(`SELECT container_key, value_json FROM container_overrides`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]model.Override{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var o model.Override
		if json.Unmarshal([]byte(raw), &o) == nil {
			out[key] = o
		}
	}
	return out, rows.Err()
}

// SaveOverride upserts one override.
func (s *Store) SaveOverride(key string, o model.Override) error {
	raw, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO container_overrides (container_key, value_json, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(container_key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		key, string(raw), time.Now().Unix())
	return err
}

// DeleteOverride removes one override.
func (s *Store) DeleteOverride(key string) error {
	_, err := s.db.Exec(`DELETE FROM container_overrides WHERE container_key = ?`, key)
	return err
}

// SaveDecision persists the latest endpoint decision for a container.
func (s *Store) SaveDecision(key string, d model.Decision) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO endpoint_decisions (container_key, value_json, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(container_key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		key, string(raw), time.Now().Unix())
	return err
}

// Decisions returns all stored decisions keyed by container key.
func (s *Store) Decisions() (map[string]model.Decision, error) {
	rows, err := s.db.Query(`SELECT container_key, value_json FROM endpoint_decisions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]model.Decision{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var d model.Decision
		if json.Unmarshal([]byte(raw), &d) == nil {
			out[key] = d
		}
	}
	return out, rows.Err()
}

// PruneDecisions removes decisions for containers that no longer exist.
func (s *Store) PruneDecisions(liveKeys map[string]bool) error {
	rows, err := s.db.Query(`SELECT container_key FROM endpoint_decisions`)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var key string
		if rows.Scan(&key) == nil && !liveKeys[key] {
			stale = append(stale, key)
		}
	}
	rows.Close()
	for _, key := range stale {
		if _, err := s.db.Exec(`DELETE FROM endpoint_decisions WHERE container_key = ?`, key); err != nil {
			return err
		}
	}
	return nil
}

// IconGet returns a cached icon.
func (s *Store) IconGet(key string) (mime string, body []byte, ok bool) {
	err := s.db.QueryRow(`SELECT mime, body FROM icon_cache WHERE cache_key = ?`, key).Scan(&mime, &body)
	if err != nil {
		return "", nil, false
	}
	return mime, body, true
}

// IconPut caches an icon body.
func (s *Store) IconPut(key, mime string, body []byte, etag string) error {
	_, err := s.db.Exec(`INSERT INTO icon_cache (cache_key, mime, body, etag, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET mime = excluded.mime, body = excluded.body, etag = excluded.etag, updated_at = excluded.updated_at`,
		key, mime, body, etag, time.Now().Unix())
	return err
}

// RecordRun appends one discovery run and keeps only the most recent 50.
func (s *Store) RecordRun(started, finished time.Time, stats any) error {
	raw, err := json.Marshal(stats)
	if err != nil {
		raw = []byte("{}")
	}
	if _, err := s.db.Exec(`INSERT INTO discovery_runs (started_at, finished_at, stats_json) VALUES (?, ?, ?)`,
		started.Unix(), finished.Unix(), string(raw)); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM discovery_runs WHERE id NOT IN (
		SELECT id FROM discovery_runs ORDER BY id DESC LIMIT 50)`)
	return err
}

func (s *Store) upsertJSON(query string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(query, string(raw), time.Now().Unix())
	return err
}
