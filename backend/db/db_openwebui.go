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
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	ParentID    string `json:"parent_id"`
	IsExpanded  bool   `json:"is_expanded"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type OpenWebUIPromptDB struct {
	ID        string `json:"id"`
	Subdomain string `json:"subdomain"`
	UserID    string `json:"user_id"`
	Command   string `json:"command"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ── Chats ──────────────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIChats(subdomain, userID string, includeArchived, includePinned bool) ([]OpenWebUIChatSummary, error) {
	query := `
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ?
	`
	if !includeArchived {
		query += ` AND archived = 0`
	}
	if !includePinned {
		query += ` AND pinned = 0`
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := d.conn.Query(query, subdomain, userID)
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

func (d *DB) GetOpenWebUIPinnedChats(subdomain, userID string) ([]OpenWebUIChatSummary, error) {
	rows, err := d.conn.Query(`
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND pinned = 1 AND archived = 0
		ORDER BY updated_at DESC
	`, subdomain, userID)
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

func (d *DB) GetOpenWebUIArchivedChats(subdomain, userID string) ([]OpenWebUIChatSummary, error) {
	rows, err := d.conn.Query(`
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND archived = 1
		ORDER BY updated_at DESC
	`, subdomain, userID)
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

func (d *DB) GetOpenWebUIChatRaw(subdomain, userID, id string) (string, string, bool, bool, string, int64, int64, error) {
	var title, chatJSON, folderID string
	var pinned, archived int
	var createdAt, updatedAt int64
	err := d.conn.QueryRow(`
		SELECT title, chat_json, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, subdomain, userID, id).Scan(&title, &chatJSON, &pinned, &archived, &folderID, &createdAt, &updatedAt)
	return title, chatJSON, pinned == 1, archived == 1, folderID, createdAt, updatedAt, err
}

func (d *DB) UpsertOpenWebUIChat(subdomain, userID, id, title, chatJSON string, folderID string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_chats (id, subdomain, user_id, title, chat_json, folder_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			chat_json = excluded.chat_json,
			folder_id = excluded.folder_id,
			updated_at = excluded.updated_at
	`, id, subdomain, userID, title, chatJSON, folderID, now, now)
	return err
}

func (d *DB) SetOpenWebUIChatPinned(subdomain, userID, id string, pinned bool) error {
	p := 0
	if pinned {
		p = 1
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET pinned = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, p, subdomain, userID, id)
	return err
}

func (d *DB) SetOpenWebUIChatArchived(subdomain, userID, id string, archived bool) error {
	a := 0
	if archived {
		a = 1
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET archived = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, a, subdomain, userID, id)
	return err
}

func (d *DB) SetOpenWebUIChatFolder(subdomain, userID, id, folderID string) error {
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET folder_id = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, folderID, subdomain, userID, id)
	return err
}

func (d *DB) DeleteOpenWebUIChat(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_chats WHERE subdomain = ? AND user_id = ? AND id = ?`, subdomain, userID, id)
	return err
}

func (d *DB) DeleteAllOpenWebUIChats(subdomain, userID string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_chats WHERE subdomain = ? AND user_id = ?`, subdomain, userID)
	return err
}

// ── Folders ────────────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIFolders(subdomain, userID string) ([]OpenWebUIFolderDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, name, COALESCE(parent_id, ''), is_expanded, created_at, updated_at
		FROM openwebui_folders
		WHERE subdomain = ? AND user_id = ?
		ORDER BY created_at ASC
	`, subdomain, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []OpenWebUIFolderDB
	for rows.Next() {
		var f OpenWebUIFolderDB
		var exp int
		if err := rows.Scan(&f.ID, &f.Subdomain, &f.UserID, &f.Name, &f.ParentID, &exp, &f.CreatedAt, &f.UpdatedAt); err != nil {
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

func (d *DB) CreateOpenWebUIFolder(subdomain, userID, id, name, parentID string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_folders (id, subdomain, user_id, name, parent_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, subdomain, userID, name, parentID, now, now)
	return err
}

func (d *DB) UpdateOpenWebUIFolderName(subdomain, userID, id, name string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`UPDATE openwebui_folders SET name = ?, updated_at = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, name, now, subdomain, userID, id)
	return err
}

func (d *DB) UpdateOpenWebUIFolderParent(subdomain, userID, id, parentID string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`UPDATE openwebui_folders SET parent_id = ?, updated_at = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, parentID, now, subdomain, userID, id)
	return err
}

func (d *DB) UpdateOpenWebUIFolderExpanded(subdomain, userID, id string, isExpanded bool) error {
	exp := 0
	if isExpanded {
		exp = 1
	}
	_, err := d.conn.Exec(`UPDATE openwebui_folders SET is_expanded = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, exp, subdomain, userID, id)
	return err
}

func (d *DB) DeleteOpenWebUIFolder(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_folders WHERE subdomain = ? AND user_id = ? AND id = ?`, subdomain, userID, id)
	return err
}

// ── Prompts ────────────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIPrompts(subdomain, userID string) ([]OpenWebUIPromptDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, command, name, content, created_at, updated_at
		FROM openwebui_prompts
		WHERE subdomain = ? AND user_id = ?
		ORDER BY updated_at DESC
	`, subdomain, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prompts []OpenWebUIPromptDB
	for rows.Next() {
		var p OpenWebUIPromptDB
		if err := rows.Scan(&p.ID, &p.Subdomain, &p.UserID, &p.Command, &p.Name, &p.Content, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		prompts = append(prompts, p)
	}
	if prompts == nil {
		prompts = []OpenWebUIPromptDB{}
	}
	return prompts, nil
}

func (d *DB) UpsertOpenWebUIPrompt(subdomain, userID, id, command, name, content string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_prompts (id, subdomain, user_id, command, name, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			command = excluded.command,
			name = excluded.name,
			content = excluded.content,
			updated_at = excluded.updated_at
	`, id, subdomain, userID, command, name, content, now, now)
	return err
}

func (d *DB) DeleteOpenWebUIPrompt(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_prompts WHERE subdomain = ? AND user_id = ? AND id = ?`, subdomain, userID, id)
	return err
}

// ── User Settings ──────────────────────────────────────────────────────────────

func (d *DB) GetOpenWebUIUserSettings(subdomain, userID string) (string, error) {
	var settings string
	err := d.conn.QueryRow(`SELECT settings_json FROM openwebui_user_settings WHERE subdomain = ? AND user_id = ?`, subdomain, userID).Scan(&settings)
	if err == sql.ErrNoRows {
		return "{}", nil
	}
	return settings, err
}

func (d *DB) SetOpenWebUIUserSettings(subdomain, userID, settingsJSON string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_user_settings (subdomain, user_id, settings_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(subdomain, user_id) DO UPDATE SET
			settings_json = excluded.settings_json,
			updated_at = excluded.updated_at
	`, subdomain, userID, settingsJSON, now)
	return err
}
