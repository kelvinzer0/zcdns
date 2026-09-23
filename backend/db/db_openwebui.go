package db

import (
	"database/sql"
	"time"
)

type OpenWebUIChatSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	FolderID  string `json:"folder_id,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type OpenWebUIFolderDB struct {
	ID          string `json:"id"`
	Subdomain   string `json:"subdomain"`
	Name        string `json:"name"`
	ParentID    string `json:"parent_id"`
	IsExpanded  bool   `json:"is_expanded"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type OpenWebUIPromptDB struct {
	ID        string `json:"id"`
	Subdomain string `json:"subdomain"`
	Command   string `json:"command"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ── Chats ──────────────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIChats(subdomain string, includeArchived, includePinned bool) ([]OpenWebUIChatSummary, error) {
	query := `
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ?
	`
	if !includeArchived {
		query += ` AND archived = 0`
	}
	if !includePinned {
		query += ` AND pinned = 0`
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := d.conn.Query(query, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []OpenWebUIChatSummary
	for rows.Next() {
		var c OpenWebUIChatSummary
		var p, a int
		if err := rows.Scan(&c.ID, &c.Title, &p, &a, &c.FolderID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.Pinned = p == 1
		c.Archived = a == 1
		chats = append(chats, c)
	}
	if chats == nil {
		chats = []OpenWebUIChatSummary{}
	}
	return chats, nil
}

func (d *DB) GetOpenWebUIPinnedChats(subdomain string) ([]OpenWebUIChatSummary, error) {
	rows, err := d.conn.Query(`
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND pinned = 1 AND archived = 0
		ORDER BY updated_at DESC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []OpenWebUIChatSummary
	for rows.Next() {
		var c OpenWebUIChatSummary
		var p, a int
		if err := rows.Scan(&c.ID, &c.Title, &p, &a, &c.FolderID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.Pinned = true
		c.Archived = false
		chats = append(chats, c)
	}
	if chats == nil {
		chats = []OpenWebUIChatSummary{}
	}
	return chats, nil
}

func (d *DB) GetOpenWebUIArchivedChats(subdomain string) ([]OpenWebUIChatSummary, error) {
	rows, err := d.conn.Query(`
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND archived = 1
		ORDER BY updated_at DESC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []OpenWebUIChatSummary
	for rows.Next() {
		var c OpenWebUIChatSummary
		var p, a int
		if err := rows.Scan(&c.ID, &c.Title, &p, &a, &c.FolderID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.Pinned = p == 1
		c.Archived = true
		chats = append(chats, c)
	}
	if chats == nil {
		chats = []OpenWebUIChatSummary{}
	}
	return chats, nil
}

func (d *DB) GetOpenWebUIChatRaw(subdomain, id string) (string, string, bool, bool, string, int64, int64, error) {
	var title, chatJSON, folderID string
	var pinned, archived int
	var createdAt, updatedAt int64
	err := d.conn.QueryRow(`
		SELECT title, chat_json, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND id = ?
	`, subdomain, id).Scan(&title, &chatJSON, &pinned, &archived, &folderID, &createdAt, &updatedAt)
	return title, chatJSON, pinned == 1, archived == 1, folderID, createdAt, updatedAt, err
}

func (d *DB) UpsertOpenWebUIChat(subdomain, id, title, chatJSON string, folderID string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_chats (id, subdomain, title, chat_json, folder_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			chat_json = excluded.chat_json,
			folder_id = excluded.folder_id,
			updated_at = excluded.updated_at
	`, id, subdomain, title, chatJSON, folderID, now, now)
	return err
}

func (d *DB) SetOpenWebUIChatPinned(subdomain, id string, pinned bool) error {
	p := 0
	if pinned {
		p = 1
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET pinned = ? WHERE subdomain = ? AND id = ?`, p, subdomain, id)
	return err
}

func (d *DB) SetOpenWebUIChatArchived(subdomain, id string, archived bool) error {
	a := 0
	if archived {
		a = 1
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET archived = ? WHERE subdomain = ? AND id = ?`, a, subdomain, id)
	return err
}

func (d *DB) SetOpenWebUIChatFolder(subdomain, id, folderID string) error {
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET folder_id = ? WHERE subdomain = ? AND id = ?`, folderID, subdomain, id)
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

// ── Folders ────────────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIFolders(subdomain string) ([]OpenWebUIFolderDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, name, COALESCE(parent_id, ''), is_expanded, created_at, updated_at
		FROM openwebui_folders
		WHERE subdomain = ?
		ORDER BY created_at ASC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []OpenWebUIFolderDB
	for rows.Next() {
		var f OpenWebUIFolderDB
		var exp int
		if err := rows.Scan(&f.ID, &f.Subdomain, &f.Name, &f.ParentID, &exp, &f.CreatedAt, &f.UpdatedAt); err != nil {
			continue
		}
		f.IsExpanded = exp == 1
		folders = append(folders, f)
	}
	if folders == nil {
		folders = []OpenWebUIFolderDB{}
	}
	return folders, nil
}

func (d *DB) CreateOpenWebUIFolder(subdomain, id, name, parentID string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_folders (id, subdomain, name, parent_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, subdomain, name, parentID, now, now)
	return err
}

func (d *DB) UpdateOpenWebUIFolderName(subdomain, id, name string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`UPDATE openwebui_folders SET name = ?, updated_at = ? WHERE subdomain = ? AND id = ?`, name, now, subdomain, id)
	return err
}

func (d *DB) UpdateOpenWebUIFolderParent(subdomain, id, parentID string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`UPDATE openwebui_folders SET parent_id = ?, updated_at = ? WHERE subdomain = ? AND id = ?`, parentID, now, subdomain, id)
	return err
}

func (d *DB) UpdateOpenWebUIFolderExpanded(subdomain, id string, isExpanded bool) error {
	exp := 0
	if isExpanded {
		exp = 1
	}
	_, err := d.conn.Exec(`UPDATE openwebui_folders SET is_expanded = ? WHERE subdomain = ? AND id = ?`, exp, subdomain, id)
	return err
}

func (d *DB) DeleteOpenWebUIFolder(subdomain, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_folders WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

// ── Prompts ────────────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIPrompts(subdomain string) ([]OpenWebUIPromptDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, command, name, content, created_at, updated_at
		FROM openwebui_prompts
		WHERE subdomain = ?
		ORDER BY updated_at DESC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prompts []OpenWebUIPromptDB
	for rows.Next() {
		var p OpenWebUIPromptDB
		if err := rows.Scan(&p.ID, &p.Subdomain, &p.Command, &p.Name, &p.Content, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		prompts = append(prompts, p)
	}
	if prompts == nil {
		prompts = []OpenWebUIPromptDB{}
	}
	return prompts, nil
}

func (d *DB) UpsertOpenWebUIPrompt(subdomain, id, command, name, content string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_prompts (id, subdomain, command, name, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			command = excluded.command,
			name = excluded.name,
			content = excluded.content,
			updated_at = excluded.updated_at
	`, id, subdomain, command, name, content, now, now)
	return err
}

func (d *DB) DeleteOpenWebUIPrompt(subdomain, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_prompts WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

// ── User Settings ──────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIUserSettings(subdomain string) (string, error) {
	var settings string
	err := d.conn.QueryRow(`SELECT settings_json FROM openwebui_user_settings WHERE subdomain = ?`, subdomain).Scan(&settings)
	if err == sql.ErrNoRows {
		return "{}", nil
	}
	return settings, err
}

func (d *DB) SetOpenWebUIUserSettings(subdomain, settingsJSON string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_user_settings (subdomain, settings_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(subdomain) DO UPDATE SET
			settings_json = excluded.settings_json,
			updated_at = excluded.updated_at
	`, subdomain, settingsJSON, now)
	return err
}
