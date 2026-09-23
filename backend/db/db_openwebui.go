package db

import (
	"time"
)

type OpenWebUIChatSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (d *DB) GetOpenWebUIChats(subdomain string) ([]OpenWebUIChatSummary, error) {
	rows, err := d.conn.Query(`
		SELECT id, title, created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ?
		ORDER BY updated_at DESC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []OpenWebUIChatSummary
	for rows.Next() {
		var c OpenWebUIChatSummary
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		chats = append(chats, c)
	}
	if chats == nil {
		chats = []OpenWebUIChatSummary{}
	}
	return chats, nil
}

func (d *DB) GetOpenWebUIChatRaw(subdomain, id string) (string, string, int64, int64, error) {
	var title, chatJSON string
	var createdAt, updatedAt int64
	err := d.conn.QueryRow(`
		SELECT title, chat_json, created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND id = ?
	`, subdomain, id).Scan(&title, &chatJSON, &createdAt, &updatedAt)
	return title, chatJSON, createdAt, updatedAt, err
}

func (d *DB) UpsertOpenWebUIChat(subdomain, id, title, chatJSON string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_chats (id, subdomain, title, chat_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			chat_json = excluded.chat_json,
			updated_at = excluded.updated_at
	`, id, subdomain, title, chatJSON, now, now)
	return err
}

func (d *DB) DeleteOpenWebUIChat(subdomain, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_chats WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

func (d *DB) DeleteAllOpenWebUIChats(subdomain string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_chats WHERE subdomain = ?`, subdomain)
	return err
}
