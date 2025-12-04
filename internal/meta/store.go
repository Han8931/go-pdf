package meta

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
)

type Metadata struct {
	Path   string
	Title  string
	Author string
	Venue  string
	Year   string
}

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initSchema() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS metadata (
  path   TEXT PRIMARY KEY,
  title  TEXT,
  author TEXT,
  venue  TEXT,
  year   TEXT
);
`)
	return err
}

func (s *Store) Get(path string) (*Metadata, error) {
	row := s.db.QueryRow(
		`SELECT path, title, author, venue, year FROM metadata WHERE path = ?`,
		path,
	)

	m := Metadata{}
	switch err := row.Scan(&m.Path, &m.Title, &m.Author, &m.Venue, &m.Year); err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		return &m, nil
	default:
		return nil, err
	}
}

func (s *Store) Upsert(m *Metadata) error {
	_, err := s.db.Exec(`
INSERT INTO metadata (path, title, author, venue, year)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
  title  = excluded.title,
  author = excluded.author,
  venue  = excluded.venue,
  year   = excluded.year
`,
		m.Path, m.Title, m.Author, m.Venue, m.Year,
	)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) MovePath(oldPath, newPath string) error {
	if oldPath == newPath {
		return nil
	}
	_, err := s.db.Exec(`UPDATE metadata SET path = ? WHERE path = ?`, newPath, oldPath)
	return err
}

func (s *Store) MoveTree(oldDir, newDir string) error {
	oldPrefix := ensureTrailingSlash(oldDir)
	newPrefix := ensureTrailingSlash(newDir)
	if oldPrefix == newPrefix {
		return nil
	}
	start := utf8.RuneCountInString(oldPrefix) + 1
	pattern := oldPrefix + "%"
	_, err := s.db.Exec(`
UPDATE metadata
SET path = ?1 || substr(path, ?2)
WHERE path LIKE ?3
`, newPrefix, start, pattern)
	return err
}

func ensureTrailingSlash(path string) string {
	if path == "" {
		return string(os.PathSeparator)
	}
	sep := string(os.PathSeparator)
	if strings.HasSuffix(path, sep) {
		return path
	}
	return path + sep
}
