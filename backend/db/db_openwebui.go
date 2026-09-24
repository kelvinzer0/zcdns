package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type OpenWebUIChatSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Pinned     bool   `json:"pinned"`
	Archived   bool   `json:"archived"`
	FolderID   string `json:"folder_id,omitempty"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	LastReadAt int64  `json:"last_read_at"`
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
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at, COALESCE(last_read_at, 0)
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
		if err := rows.Scan(&c.ID, &c.Title, &p, &a, &c.FolderID, &c.CreatedAt, &c.UpdatedAt, &c.LastReadAt); err != nil {
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
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at, COALESCE(last_read_at, 0)
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
		if err := rows.Scan(&c.ID, &c.Title, &p, &a, &c.FolderID, &c.CreatedAt, &c.UpdatedAt, &c.LastReadAt); err != nil {
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
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at, COALESCE(last_read_at, 0)
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
		if err := rows.Scan(&c.ID, &c.Title, &p, &a, &c.FolderID, &c.CreatedAt, &c.UpdatedAt, &c.LastReadAt); err != nil {
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

func (d *DB) UpdateOpenWebUIChatTitle(subdomain, userID, id, title string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET title = ?, updated_at = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, title, now, subdomain, userID, id)
	return err
}

// UpdateOpenWebUIMessageInChat modifies or appends to a message inside openwebui_chats chat_json
func (d *DB) UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID string, updateFn func(msg map[string]interface{}) map[string]interface{}) error {
	var chatJSON string
	err := d.conn.QueryRow(`SELECT chat_json FROM openwebui_chats WHERE subdomain = ? AND user_id = ? AND id = ?`, subdomain, userID, chatID).Scan(&chatJSON)
	if err != nil {
		return err
	}
	var chatObj map[string]interface{}
	if err := json.Unmarshal([]byte(chatJSON), &chatObj); err != nil {
		return err
	}
	history, _ := chatObj["history"].(map[string]interface{})
	if history == nil {
		history = make(map[string]interface{})
		chatObj["history"] = history
	}
	messages, _ := history["messages"].(map[string]interface{})
	if messages == nil {
		messages = make(map[string]interface{})
		history["messages"] = messages
	}
	targetMsg, _ := messages[messageID].(map[string]interface{})
	if targetMsg == nil {
		targetMsg = make(map[string]interface{})
	}
	messages[messageID] = updateFn(targetMsg)
	newChatJSON, err := json.Marshal(chatObj)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = d.conn.Exec(`UPDATE openwebui_chats SET chat_json = ?, updated_at = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, string(newChatJSON), now, subdomain, userID, chatID)
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

func (d *DB) GetOpenWebUIFolderByID(subdomain, userID, id string) (*OpenWebUIFolderDB, error) {
	var f OpenWebUIFolderDB
	var parentID sql.NullString
	var isExpanded int
	err := d.conn.QueryRow(`
		SELECT id, subdomain, user_id, name, parent_id, is_expanded, created_at, updated_at
		FROM openwebui_folders
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, subdomain, userID, id).Scan(&f.ID, &f.Subdomain, &f.UserID, &f.Name, &parentID, &isExpanded, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		f.ParentID = parentID.String
	}
	f.IsExpanded = isExpanded == 1
	return &f, nil
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

// ── Chat Read Status & Unread Counts ──────────────────────────────────────────

func (d *DB) UpdateOpenWebUIChatLastReadAt(subdomain, userID, chatID string) (int64, bool, error) {
	now := time.Now().Unix()
	var updatedAt int64
	var lastReadAt int64
	err := d.conn.QueryRow(`
		SELECT updated_at, last_read_at
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, subdomain, userID, chatID).Scan(&updatedAt, &lastReadAt)
	if err != nil {
		return now, false, err
	}

	wasUnread := updatedAt > lastReadAt

	_, err = d.conn.Exec(`
		UPDATE openwebui_chats
		SET last_read_at = ?
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, now, subdomain, userID, chatID)
	if err != nil {
		return now, wasUnread, err
	}

	return now, wasUnread, nil
}

func (d *DB) CountOpenWebUIUnreadByFolder(subdomain, userID string) (map[string]int, error) {
	rows, err := d.conn.Query(`
		SELECT COALESCE(folder_id, ''), COUNT(*)
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND archived = 0 AND updated_at > last_read_at
		GROUP BY folder_id
	`, subdomain, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var folderID string
		var count int
		if err := rows.Scan(&folderID, &count); err == nil && folderID != "" {
			counts[folderID] = count
		}
	}
	return counts, nil
}

func (d *DB) MarkAllOpenWebUIChatsRead(subdomain, userID string) (int64, error) {
	now := time.Now().Unix()
	res, err := d.conn.Exec(`
		UPDATE openwebui_chats
		SET last_read_at = ?
		WHERE subdomain = ? AND user_id = ? AND archived = 0 AND updated_at > last_read_at
	`, now, subdomain, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (d *DB) MarkOpenWebUIChatUnread(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`
		UPDATE openwebui_chats
		SET last_read_at = 0
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, subdomain, userID, id)
	return err
}

func (d *DB) MarkOpenWebUIChatsReadByFolderIDs(subdomain, userID string, folderIDs []string) (int64, error) {
	if len(folderIDs) == 0 {
		return 0, nil
	}
	now := time.Now().Unix()
	placeholders := make([]string, len(folderIDs))
	args := []interface{}{now, subdomain, userID}
	for i, fID := range folderIDs {
		placeholders[i] = "?"
		args = append(args, fID)
	}
	query := fmt.Sprintf(`
		UPDATE openwebui_chats
		SET last_read_at = ?
		WHERE subdomain = ? AND user_id = ? AND archived = 0 AND updated_at > last_read_at AND folder_id IN (%s)
	`, strings.Join(placeholders, ","))
	res, err := d.conn.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (d *DB) GetOpenWebUISubtreeFolderIDs(subdomain, userID, rootFolderID string) ([]string, error) {
	folders, err := d.GetOpenWebUIFolders(subdomain, userID)
	if err != nil {
		return []string{rootFolderID}, err
	}
	// Build parent -> children map
	childrenOf := make(map[string][]string)
	for _, f := range folders {
		if f.ParentID != "" {
			childrenOf[f.ParentID] = append(childrenOf[f.ParentID], f.ID)
		}
	}
	var result []string
	seen := make(map[string]bool)
	queue := []string{rootFolderID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if seen[curr] {
			continue
		}
		seen[curr] = true
		result = append(result, curr)
		for _, childID := range childrenOf[curr] {
			if !seen[childID] {
				queue = append(queue, childID)
			}
		}
	}
	return result, nil
}

// ── Files & Uploads ────────────────────────────────────────────────────────────

type OpenWebUIFileDB struct {
	ID          string `json:"id"`
	Subdomain   string `json:"subdomain"`
	UserID      string `json:"user_id"`
	Hash        string `json:"hash"`
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	DataJSON    string `json:"data_json"`
	MetaJSON    string `json:"meta_json"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (d *DB) InsertOpenWebUIFile(f OpenWebUIFileDB) error {
	now := time.Now().Unix()
	if f.CreatedAt == 0 {
		f.CreatedAt = now
	}
	if f.UpdatedAt == 0 {
		f.UpdatedAt = now
	}
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_files (
			id, subdomain, user_id, hash, filename, path, content_type, size, data_json, meta_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			filename = excluded.filename,
			path = excluded.path,
			content_type = excluded.content_type,
			size = excluded.size,
			data_json = excluded.data_json,
			meta_json = excluded.meta_json,
			updated_at = excluded.updated_at
	`, f.ID, f.Subdomain, f.UserID, f.Hash, f.Filename, f.Path, f.ContentType, f.Size, f.DataJSON, f.MetaJSON, f.CreatedAt, f.UpdatedAt)
	return err
}

func (d *DB) GetOpenWebUIFileByID(subdomain, id string) (*OpenWebUIFileDB, error) {
	row := d.conn.QueryRow(`
		SELECT id, subdomain, user_id, hash, filename, path, content_type, size, data_json, meta_json, created_at, updated_at
		FROM openwebui_files
		WHERE subdomain = ? AND id = ?
	`, subdomain, id)

	var f OpenWebUIFileDB
	err := row.Scan(&f.ID, &f.Subdomain, &f.UserID, &f.Hash, &f.Filename, &f.Path, &f.ContentType, &f.Size, &f.DataJSON, &f.MetaJSON, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (d *DB) GetOpenWebUIFiles(subdomain, userID string, skip, limit int) ([]OpenWebUIFileDB, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
	}

	var total int64
	countErr := d.conn.QueryRow(`
		SELECT COUNT(*) FROM openwebui_files WHERE subdomain = ? AND (user_id = ? OR user_id = 'default')
	`, subdomain, userID).Scan(&total)
	if countErr != nil {
		total = 0
	}

	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, hash, filename, path, content_type, size, data_json, meta_json, created_at, updated_at
		FROM openwebui_files
		WHERE subdomain = ? AND (user_id = ? OR user_id = 'default')
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, subdomain, userID, limit, skip)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []OpenWebUIFileDB
	for rows.Next() {
		var f OpenWebUIFileDB
		if err := rows.Scan(&f.ID, &f.Subdomain, &f.UserID, &f.Hash, &f.Filename, &f.Path, &f.ContentType, &f.Size, &f.DataJSON, &f.MetaJSON, &f.CreatedAt, &f.UpdatedAt); err == nil {
			files = append(files, f)
		}
	}
	if files == nil {
		files = []OpenWebUIFileDB{}
	}
	return files, total, nil
}

func (d *DB) SearchOpenWebUIFiles(subdomain, userID, pattern string, skip, limit int) ([]OpenWebUIFileDB, error) {
	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
	}
	// Convert glob pattern (e.g. *test*) to SQL LIKE pattern (%test%)
	likePattern := "%"
	if pattern != "" && pattern != "*" {
		likePattern = strings.ReplaceAll(pattern, "*", "%")
		if !strings.HasPrefix(likePattern, "%") && !strings.HasSuffix(likePattern, "%") {
			likePattern = "%" + likePattern + "%"
		}
	}

	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, hash, filename, path, content_type, size, data_json, meta_json, created_at, updated_at
		FROM openwebui_files
		WHERE subdomain = ? AND (user_id = ? OR user_id = 'default') AND (filename LIKE ? OR meta_json LIKE ?)
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, subdomain, userID, likePattern, likePattern, limit, skip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []OpenWebUIFileDB
	for rows.Next() {
		var f OpenWebUIFileDB
		if err := rows.Scan(&f.ID, &f.Subdomain, &f.UserID, &f.Hash, &f.Filename, &f.Path, &f.ContentType, &f.Size, &f.DataJSON, &f.MetaJSON, &f.CreatedAt, &f.UpdatedAt); err == nil {
			files = append(files, f)
		}
	}
	if files == nil {
		files = []OpenWebUIFileDB{}
	}
	return files, nil
}

func (d *DB) CountOpenWebUIFiles(subdomain, userID string) (int64, error) {
	var count int64
	err := d.conn.QueryRow(`
		SELECT COUNT(*) FROM openwebui_files WHERE subdomain = ? AND (user_id = ? OR user_id = 'default')
	`, subdomain, userID).Scan(&count)
	return count, err
}

func (d *DB) UpdateOpenWebUIFileData(subdomain, id, dataJSON string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		UPDATE openwebui_files
		SET data_json = ?, updated_at = ?
		WHERE subdomain = ? AND id = ?
	`, dataJSON, now, subdomain, id)
	return err
}

func (d *DB) UpdateOpenWebUIFileName(subdomain, id, filename string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		UPDATE openwebui_files
		SET filename = ?, updated_at = ?
		WHERE subdomain = ? AND id = ?
	`, filename, now, subdomain, id)
	return err
}

func (d *DB) DeleteOpenWebUIFile(subdomain, id string) error {
	_, err := d.conn.Exec(`
		DELETE FROM openwebui_files
		WHERE subdomain = ? AND id = ?
	`, subdomain, id)
	return err
}

func (d *DB) DeleteAllOpenWebUIFiles(subdomain, userID string) error {
	_, err := d.conn.Exec(`
		DELETE FROM openwebui_files
		WHERE subdomain = ? AND (user_id = ? OR user_id = 'default')
	`, subdomain, userID)
	return err
}

// ── User Profiles & Avatars ────────────────────────────────────────────────────

type OpenWebUIUserProfileDB struct {
	Subdomain       string `json:"subdomain"`
	UserID          string `json:"user_id"`
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"`
	Bio             string `json:"bio"`
	Gender          string `json:"gender"`
	DateOfBirth     string `json:"date_of_birth"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (d *DB) GetOpenWebUIUserProfile(subdomain, userID string) (*OpenWebUIUserProfileDB, error) {
	row := d.conn.QueryRow(`
		SELECT subdomain, user_id, name, profile_image_url, bio, gender, date_of_birth, updated_at
		FROM openwebui_user_profiles
		WHERE subdomain = ? AND user_id = ?
	`, subdomain, userID)

	var p OpenWebUIUserProfileDB
	if err := row.Scan(&p.Subdomain, &p.UserID, &p.Name, &p.ProfileImageURL, &p.Bio, &p.Gender, &p.DateOfBirth, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) UpsertOpenWebUIUserProfile(subdomain, userID, name, profileImageURL, bio, gender, dateOfBirth string) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_user_profiles (
			subdomain, user_id, name, profile_image_url, bio, gender, date_of_birth, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(subdomain, user_id) DO UPDATE SET
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE openwebui_user_profiles.name END,
			profile_image_url = CASE WHEN excluded.profile_image_url != '' THEN excluded.profile_image_url ELSE openwebui_user_profiles.profile_image_url END,
			bio = excluded.bio,
			gender = excluded.gender,
			date_of_birth = excluded.date_of_birth,
			updated_at = excluded.updated_at
	`, subdomain, userID, name, profileImageURL, bio, gender, dateOfBirth, now)
	return err
}

