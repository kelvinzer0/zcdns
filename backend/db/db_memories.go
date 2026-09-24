package db

import (
	"time"
)

// OpenWebUIMemoryDB represents a user memory row in SQLite
type OpenWebUIMemoryDB struct {
	ID        string `json:"id"`
	Subdomain string `json:"subdomain"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// GetOpenWebUIMemories returns all memories for a user
func (d *DB) GetOpenWebUIMemories(subdomain, userID string) ([]OpenWebUIMemoryDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, content, type, path, created_at, updated_at
		FROM openwebui_memories
		WHERE subdomain = ? AND user_id = ?
		ORDER BY created_at DESC
	`, subdomain, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []OpenWebUIMemoryDB
	for rows.Next() {
		var m OpenWebUIMemoryDB
		if err := rows.Scan(&m.ID, &m.Subdomain, &m.UserID, &m.Content, &m.Type, &m.Path, &m.CreatedAt, &m.UpdatedAt); err != nil {
			continue
		}
		memories = append(memories, m)
	}
	if memories == nil {
		memories = []OpenWebUIMemoryDB{}
	}
	return memories, nil
}

// SearchOpenWebUIMemories searches memories by keyword query
func (d *DB) SearchOpenWebUIMemories(subdomain, userID, query string) ([]OpenWebUIMemoryDB, error) {
	pattern := "%" + query + "%"
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, content, type, path, created_at, updated_at
		FROM openwebui_memories
		WHERE subdomain = ? AND user_id = ? AND (content LIKE ? OR path LIKE ?)
		ORDER BY updated_at DESC
		LIMIT 20
	`, subdomain, userID, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []OpenWebUIMemoryDB
	for rows.Next() {
		var m OpenWebUIMemoryDB
		if err := rows.Scan(&m.ID, &m.Subdomain, &m.UserID, &m.Content, &m.Type, &m.Path, &m.CreatedAt, &m.UpdatedAt); err != nil {
			continue
		}
		memories = append(memories, m)
	}
	if memories == nil {
		memories = []OpenWebUIMemoryDB{}
	}
	return memories, nil
}

// InsertOpenWebUIMemory inserts a new memory
func (d *DB) InsertOpenWebUIMemory(m OpenWebUIMemoryDB) error {
	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	if m.UpdatedAt == 0 {
		m.UpdatedAt = now
	}
	if m.Type == "" {
		m.Type = "context"
	}
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_memories (id, subdomain, user_id, content, type, path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			type = excluded.type,
			path = excluded.path,
			updated_at = excluded.updated_at
	`, m.ID, m.Subdomain, m.UserID, m.Content, m.Type, m.Path, m.CreatedAt, m.UpdatedAt)
	return err
}

// UpdateOpenWebUIMemory updates an existing memory
func (d *DB) UpdateOpenWebUIMemory(subdomain, userID, id, content string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		UPDATE openwebui_memories
		SET content = ?, updated_at = ?
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, content, now, subdomain, userID, id)
	return err
}

// DeleteOpenWebUIMemory removes a specific memory
func (d *DB) DeleteOpenWebUIMemory(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`
		DELETE FROM openwebui_memories
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, subdomain, userID, id)
	return err
}

// DeleteAllOpenWebUIMemories removes all memories for a user
func (d *DB) DeleteAllOpenWebUIMemories(subdomain, userID string) error {
	_, err := d.conn.Exec(`
		DELETE FROM openwebui_memories
		WHERE subdomain = ? AND user_id = ?
	`, subdomain, userID)
	return err
}
