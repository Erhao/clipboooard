package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type BanRecord struct {
	IP        string
	FailCount int
	BanUntil  time.Time
}

type Clip struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content,omitempty"`
	Filename  string    `json:"filename,omitempty"`
	FileSize  int64     `json:"fileSize,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS clips (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		content TEXT DEFAULT '',
		filename TEXT DEFAULT '',
		file_size INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS bans (
		ip TEXT PRIMARY KEY,
		fail_count INTEGER DEFAULT 0,
		ban_until DATETIME
	)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS auth (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateClip(c *Clip) error {
	_, err := s.db.Exec(
		"INSERT INTO clips (id, type, content, filename, file_size, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		c.ID, c.Type, c.Content, c.Filename, c.FileSize, c.CreatedAt,
	)
	return err
}

func (s *Store) GetClip(id string) (*Clip, error) {
	c := &Clip{}
	err := s.db.QueryRow(
		"SELECT id, type, content, filename, file_size, created_at FROM clips WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Type, &c.Content, &c.Filename, &c.FileSize, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetLatestClip() (*Clip, error) {
	c := &Clip{}
	err := s.db.QueryRow(
		"SELECT id, type, content, filename, file_size, created_at FROM clips ORDER BY created_at DESC LIMIT 1",
	).Scan(&c.ID, &c.Type, &c.Content, &c.Filename, &c.FileSize, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetSecret() (string, error) {
	var val string
	err := s.db.QueryRow("SELECT value FROM auth WHERE key='secret'").Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (s *Store) SetSecret(secret string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO auth (key, value) VALUES ('secret', ?)", secret)
	return err
}

func (s *Store) CheckBan(ip string) (*BanRecord, bool) {
	b := &BanRecord{}
	err := s.db.QueryRow("SELECT ip, fail_count, ban_until FROM bans WHERE ip=?", ip).Scan(&b.IP, &b.FailCount, &b.BanUntil)
	if err != nil {
		return nil, false
	}
	if b.BanUntil.After(time.Now()) {
		return b, true
	}
	return nil, false
}

func (s *Store) RecordFail(ip string) int {
	s.db.Exec("INSERT INTO bans (ip, fail_count) VALUES (?, 1) ON CONFLICT(ip) DO UPDATE SET fail_count = bans.fail_count + 1", ip)
	var count int
	s.db.QueryRow("SELECT fail_count FROM bans WHERE ip=?", ip).Scan(&count)
	return count
}

func (s *Store) SetBan(ip string, count int, until time.Time) {
	s.db.Exec("UPDATE bans SET fail_count=?, ban_until=? WHERE ip=?", count, until, ip)
}

func (s *Store) ResetBan(ip string) {
	s.db.Exec("DELETE FROM bans WHERE ip=?", ip)
}

func (s *Store) ListClips(limit int) ([]Clip, error) {
	rows, err := s.db.Query(
		"SELECT id, type, content, filename, file_size, created_at FROM clips ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clips []Clip
	for rows.Next() {
		var c Clip
		if err := rows.Scan(&c.ID, &c.Type, &c.Content, &c.Filename, &c.FileSize, &c.CreatedAt); err != nil {
			return nil, err
		}
		clips = append(clips, c)
	}
	return clips, nil
}
