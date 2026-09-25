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

func (d *DB) GetOpenWebUIChats(subdomain, userID string, includeArchived, includePinned, includeFolders bool) ([]OpenWebUIChatSummary, error) {
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
	if !includeFolders {
		query += ` AND (folder_id IS NULL OR folder_id = '')`
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

func (d *DB) GetOpenWebUIChatsByFolder(subdomain, userID, folderID string) ([]OpenWebUIChatSummary, error) {
	query := `
		SELECT id, title, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at, COALESCE(last_read_at, 0)
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND folder_id = ? AND archived = 0
		ORDER BY updated_at DESC
	`
	rows, err := d.conn.Query(query, subdomain, userID, folderID)
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
	// 1. Try reading from individual conversation SQLite database
	if d.chatPool != nil {
		chatDB, _, poolErr := d.chatPool.GetChatDB(subdomain, userID, id)
		if poolErr == nil && chatDB != nil {
			var title, chatJSON string
			var folderID sql.NullString
			var pinned, archived int
			var createdAt, updatedAt int64
			qErr := chatDB.QueryRow(`
				SELECT title, chat_json, pinned, archived, folder_id, created_at, updated_at
				FROM chat_session WHERE id = ?
			`, id).Scan(&title, &chatJSON, &pinned, &archived, &folderID, &createdAt, &updatedAt)
			if qErr == nil && chatJSON != "" && chatJSON != "{}" {
				fID := ""
				if folderID.Valid {
					fID = folderID.String
				}
				return title, chatJSON, pinned == 1, archived == 1, fID, createdAt, updatedAt, nil
			}
		}
	}

	// 2. Fallback to main zcdns.db
	var title, chatJSON, folderID string
	var pinned, archived int
	var createdAt, updatedAt int64
	err := d.conn.QueryRow(`
		SELECT title, chat_json, pinned, archived, COALESCE(folder_id, ''), created_at, updated_at
		FROM openwebui_chats
		WHERE subdomain = ? AND user_id = ? AND id = ?
	`, subdomain, userID, id).Scan(&title, &chatJSON, &pinned, &archived, &folderID, &createdAt, &updatedAt)
	if err != nil {
		return "", "", false, false, "", 0, 0, err
	}

	// 3. Migrate to conversation database so subsequent operations use the isolated DB
	if d.chatPool != nil && chatJSON != "" && chatJSON != "{}" {
		if chatDB, _, poolErr := d.chatPool.GetChatDB(subdomain, userID, id); poolErr == nil && chatDB != nil {
			_, _ = chatDB.Exec(`
				INSERT INTO chat_session (id, subdomain, user_id, title, folder_id, pinned, archived, chat_json, created_at, updated_at, last_read_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
				ON CONFLICT(id) DO UPDATE SET
					chat_json = excluded.chat_json,
					updated_at = excluded.updated_at
			`, id, subdomain, userID, title, folderID, pinned, archived, chatJSON, createdAt, updatedAt)
			d.chatPool.SyncMessagesTree(chatDB, chatJSON)
		}
	}

	return title, chatJSON, pinned == 1, archived == 1, folderID, createdAt, updatedAt, nil
}

// MergeHistory reconciles an incoming DAG history with the existing DAG history.
// It ensures that partial updates, stale clients, or new branches never delete existing messages.
func MergeHistory(existingHistory, incomingHistory map[string]interface{}) map[string]interface{} {
	existingMsgs, _ := existingHistory["messages"].(map[string]interface{})
	incomingMsgs, _ := incomingHistory["messages"].(map[string]interface{})

	if existingMsgs == nil {
		existingMsgs = make(map[string]interface{})
	}
	if incomingMsgs == nil {
		incomingMsgs = make(map[string]interface{})
	}

	mergedMsgs := make(map[string]interface{})

	// 1. Copy all existing messages first
	for id, m := range existingMsgs {
		if mMap, ok := m.(map[string]interface{}); ok {
			cloned := make(map[string]interface{}, len(mMap)+1)
			for k, v := range mMap {
				cloned[k] = v
			}
			cloned["childrenIds"] = []interface{}{}
			mergedMsgs[id] = cloned
		}
	}

	// 2. Merge incoming messages
	for id, m := range incomingMsgs {
		if mMap, ok := m.(map[string]interface{}); ok {
			existingMsg, exists := mergedMsgs[id].(map[string]interface{})
			if exists && existingMsg != nil {
				newContent, _ := mMap["content"].(string)
				oldContent, _ := existingMsg["content"].(string)
				for k, v := range mMap {
					existingMsg[k] = v
				}
				// Safeguard: never overwrite non-empty content with empty string unless explicitly intentional
				if strings.TrimSpace(newContent) == "" && strings.TrimSpace(oldContent) != "" {
					existingMsg["content"] = oldContent
				}
				existingMsg["childrenIds"] = []interface{}{}
			} else {
				cloned := make(map[string]interface{}, len(mMap)+1)
				for k, v := range mMap {
					cloned[k] = v
				}
				cloned["childrenIds"] = []interface{}{}
				mergedMsgs[id] = cloned
			}
		}
	}

	// 3. Rebuild childrenIds from parentId
	for id, m := range mergedMsgs {
		if mMap, ok := m.(map[string]interface{}); ok {
			if pid, ok := mMap["parentId"].(string); ok && pid != "" {
				if parent, exists := mergedMsgs[pid].(map[string]interface{}); exists && parent != nil {
					cIds, _ := parent["childrenIds"].([]interface{})
					found := false
					for _, c := range cIds {
						if c == id {
							found = true
							break
						}
					}
					if !found {
						parent["childrenIds"] = append(cIds, id)
					}
				}
			}
		}
	}

	// 4. Resolve currentId
	var currentID interface{}
	if curr, ok := incomingHistory["currentId"].(string); ok && curr != "" {
		if _, exists := mergedMsgs[curr]; exists {
			currentID = curr
		}
	}
	if currentID == nil {
		if curr, ok := existingHistory["currentId"].(string); ok && curr != "" {
			if _, exists := mergedMsgs[curr]; exists {
				currentID = curr
			}
		}
	}

	// 5. Construct merged history
	result := make(map[string]interface{})
	for k, v := range existingHistory {
		result[k] = v
	}
	for k, v := range incomingHistory {
		result[k] = v
	}
	result["messages"] = mergedMsgs
	if currentID != nil {
		result["currentId"] = currentID
	}

	return result
}

// MergeUpdateOpenWebUIChat safely merges incoming chat updates into existing chat state.
// This prevents partial updates (e.g. setting params, title, tags) or switching models from wiping out messages.
func (d *DB) MergeUpdateOpenWebUIChat(subdomain, userID, id string, incomingChat map[string]interface{}, folderID string) (map[string]interface{}, error) {
	now := time.Now().Unix()

	// 1. Get existing chat
	existingTitle, existingJSON, pinned, archived, existingFolderID, createdAt, _, _ := d.GetOpenWebUIChatRaw(subdomain, userID, id)

	var storedObj map[string]interface{}
	if existingJSON != "" && existingJSON != "{}" {
		_ = json.Unmarshal([]byte(existingJSON), &storedObj)
	}
	if storedObj == nil {
		storedObj = make(map[string]interface{})
	}

	storedChat := storedObj
	if inner, ok := storedObj["chat"].(map[string]interface{}); ok && inner != nil {
		storedChat = inner
	}

	incomingTarget := incomingChat
	if inner, ok := incomingChat["chat"].(map[string]interface{}); ok && inner != nil {
		incomingTarget = inner
	}

	// 2. Merge top-level fields
	updatedChat := make(map[string]interface{})
	for k, v := range storedChat {
		updatedChat[k] = v
	}
	for k, v := range incomingTarget {
		if k == "history" {
			continue // Handled below
		}
		updatedChat[k] = v
	}

	// 3. Merge history
	incomingHist, hasIncomingHist := incomingTarget["history"].(map[string]interface{})
	existingHist, _ := storedChat["history"].(map[string]interface{})

	if hasIncomingHist && incomingHist != nil {
		updatedChat["history"] = MergeHistory(existingHist, incomingHist)
	} else if existingHist != nil {
		updatedChat["history"] = existingHist
	}

	// 4. Resolve Title
	finalTitle := existingTitle
	if t, ok := incomingTarget["title"].(string); ok && strings.TrimSpace(t) != "" {
		finalTitle = strings.TrimSpace(t)
	} else if t, ok := incomingChat["title"].(string); ok && strings.TrimSpace(t) != "" {
		finalTitle = strings.TrimSpace(t)
	}
	if finalTitle == "" {
		finalTitle = "Chat"
	}
	updatedChat["title"] = finalTitle

	// 5. Resolve FolderID
	finalFolderID := existingFolderID
	if folderID != "" {
		finalFolderID = folderID
	} else if fid, ok := incomingTarget["folder_id"].(string); ok && fid != "" {
		finalFolderID = fid
	} else if fid, ok := incomingChat["folder_id"].(string); ok && fid != "" {
		finalFolderID = fid
	}

	// 6. Ensure id and timestamps
	updatedChat["id"] = id
	if createdAt == 0 {
		createdAt = now
	}

	// Preserve format structure (wrapped in { "chat": ... } or root)
	var finalObj map[string]interface{}
	if _, ok := storedObj["chat"]; ok || incomingChat["chat"] != nil {
		finalObj = map[string]interface{}{
			"chat": updatedChat,
		}
		if v, ok := incomingChat["variables"]; ok {
			finalObj["variables"] = v
		} else if v, ok := storedObj["variables"]; ok {
			finalObj["variables"] = v
		}
	} else {
		finalObj = updatedChat
	}

	newJSONBytes, err := json.Marshal(finalObj)
	if err != nil {
		return nil, err
	}
	newJSON := string(newJSONBytes)

	// 7. Write to isolated conversation SQLite database
	if d.chatPool != nil {
		if chatDB, _, poolErr := d.chatPool.GetChatDB(subdomain, userID, id); poolErr == nil && chatDB != nil {
			p := 0
			if pinned {
				p = 1
			}
			a := 0
			if archived {
				a = 1
			}
			_, _ = chatDB.Exec(`
				INSERT INTO chat_session (id, subdomain, user_id, title, folder_id, pinned, archived, chat_json, created_at, updated_at, last_read_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
				ON CONFLICT(id) DO UPDATE SET
					title = excluded.title,
					folder_id = excluded.folder_id,
					chat_json = excluded.chat_json,
					updated_at = excluded.updated_at
			`, id, subdomain, userID, finalTitle, finalFolderID, p, a, newJSON, createdAt, now)

			d.chatPool.SyncMessagesTree(chatDB, newJSON)
		}
	}

	// 8. Update central index
	_, err = d.conn.Exec(`
		INSERT INTO openwebui_chats (id, subdomain, user_id, title, chat_json, folder_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			chat_json = excluded.chat_json,
			folder_id = excluded.folder_id,
			updated_at = excluded.updated_at
	`, id, subdomain, userID, finalTitle, newJSON, finalFolderID, createdAt, now)

	return finalObj, err
}

func (d *DB) UpsertOpenWebUIChat(subdomain, userID, id, title, chatJSON string, folderID string) error {
	var incoming map[string]interface{}
	if err := json.Unmarshal([]byte(chatJSON), &incoming); err == nil && incoming != nil {
		if title != "" {
			incoming["title"] = title
		}
		_, err := d.MergeUpdateOpenWebUIChat(subdomain, userID, id, incoming, folderID)
		return err
	}

	now := time.Now().Unix()

	// Fallback write if not valid JSON
	if d.chatPool != nil {
		if chatDB, _, err := d.chatPool.GetChatDB(subdomain, userID, id); err == nil && chatDB != nil {
			_, _ = chatDB.Exec(`
				INSERT INTO chat_session (id, subdomain, user_id, title, folder_id, pinned, archived, chat_json, created_at, updated_at, last_read_at)
				VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?, ?, 0)
				ON CONFLICT(id) DO UPDATE SET
					title = excluded.title,
					folder_id = excluded.folder_id,
					chat_json = excluded.chat_json,
					updated_at = excluded.updated_at
			`, id, subdomain, userID, title, folderID, chatJSON, now, now)

			d.chatPool.SyncMessagesTree(chatDB, chatJSON)
		}
	}

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
	if d.chatPool != nil {
		if chatDB, _, err := d.chatPool.GetChatDB(subdomain, userID, id); err == nil && chatDB != nil {
			_, _ = chatDB.Exec(`UPDATE chat_session SET pinned = ? WHERE id = ?`, p, id)
		}
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET pinned = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, p, subdomain, userID, id)
	return err
}

func (d *DB) SetOpenWebUIChatArchived(subdomain, userID, id string, archived bool) error {
	a := 0
	if archived {
		a = 1
	}
	if d.chatPool != nil {
		if chatDB, _, err := d.chatPool.GetChatDB(subdomain, userID, id); err == nil && chatDB != nil {
			_, _ = chatDB.Exec(`UPDATE chat_session SET archived = ? WHERE id = ?`, a, id)
		}
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET archived = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, a, subdomain, userID, id)
	return err
}

func (d *DB) SetOpenWebUIChatFolder(subdomain, userID, id, folderID string) error {
	if d.chatPool != nil {
		if chatDB, _, err := d.chatPool.GetChatDB(subdomain, userID, id); err == nil && chatDB != nil {
			_, _ = chatDB.Exec(`UPDATE chat_session SET folder_id = ? WHERE id = ?`, folderID, id)
		}
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET folder_id = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, folderID, subdomain, userID, id)
	return err
}

func (d *DB) DeleteOpenWebUIChat(subdomain, userID, id string) error {
	if d.chatPool != nil {
		_ = d.chatPool.DeleteChatDB(subdomain, userID, id)
	}
	_, err := d.conn.Exec(`DELETE FROM openwebui_chats WHERE subdomain = ? AND user_id = ? AND id = ?`, subdomain, userID, id)
	return err
}

func (d *DB) DeleteAllOpenWebUIChats(subdomain, userID string) error {
	if d.chatPool != nil {
		_ = d.chatPool.DeleteUserChats(subdomain, userID)
	}
	_, err := d.conn.Exec(`DELETE FROM openwebui_chats WHERE subdomain = ? AND user_id = ?`, subdomain, userID)
	return err
}

func (d *DB) UpdateOpenWebUIChatTitle(subdomain, userID, id, title string) error {
	now := time.Now().Unix()
	if d.chatPool != nil {
		if chatDB, _, err := d.chatPool.GetChatDB(subdomain, userID, id); err == nil && chatDB != nil {
			_, _ = chatDB.Exec(`UPDATE chat_session SET title = ?, updated_at = ? WHERE id = ?`, title, now, id)
		}
	}
	_, err := d.conn.Exec(`UPDATE openwebui_chats SET title = ?, updated_at = ? WHERE subdomain = ? AND user_id = ? AND id = ?`, title, now, subdomain, userID, id)
	return err
}

// UpdateOpenWebUIMessageInChat modifies or appends to a message inside isolated per-conversation SQLite database
func (d *DB) UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID string, updateFn func(msg map[string]interface{}) map[string]interface{}) error {
	now := time.Now().Unix()

	// 1. If chatPool is active, operate on the isolated conversation database
	if d.chatPool != nil {
		chatDB, _, poolErr := d.chatPool.GetChatDB(subdomain, userID, chatID)
		if poolErr == nil && chatDB != nil {
			var chatJSON, title string
			var folderID sql.NullString
			var pinned, archived int
			var createdAt, updatedAt, lastReadAt int64

			qErr := chatDB.QueryRow(`
				SELECT title, folder_id, pinned, archived, chat_json, created_at, updated_at, last_read_at
				FROM chat_session WHERE id = ?
			`, chatID).Scan(&title, &folderID, &pinned, &archived, &chatJSON, &createdAt, &updatedAt, &lastReadAt)

			if qErr == sql.ErrNoRows || chatJSON == "" || chatJSON == "{}" {
				// Fallback to legacy single-db record if this chat hasn't been migrated yet
				var mainChatJSON string
				mErr := d.conn.QueryRow(`
					SELECT title, folder_id, pinned, archived, chat_json, created_at, updated_at, last_read_at
					FROM openwebui_chats WHERE subdomain = ? AND user_id = ? AND id = ?
				`, subdomain, userID, chatID).Scan(&title, &folderID, &pinned, &archived, &mainChatJSON, &createdAt, &updatedAt, &lastReadAt)
				if mErr == nil {
					chatJSON = mainChatJSON
				} else {
					chatJSON = "{}"
				}
			} else if qErr != nil {
				return qErr
			}

			var chatObj map[string]interface{}
			if err := json.Unmarshal([]byte(chatJSON), &chatObj); err != nil {
				chatObj = make(map[string]interface{})
			}
			targetMap := chatObj
			if inner, ok := chatObj["chat"].(map[string]interface{}); ok && inner != nil {
				targetMap = inner
			}
			history, _ := targetMap["history"].(map[string]interface{})
			if history == nil {
				history = make(map[string]interface{})
				targetMap["history"] = history
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
			updatedMsg := updateFn(targetMsg)
			messages[messageID] = updatedMsg

			// Ensure parent's childrenIds includes this messageID
			if pid, ok := updatedMsg["parentId"].(string); ok && pid != "" {
				if parent, ok := messages[pid].(map[string]interface{}); ok && parent != nil {
					cIds, _ := parent["childrenIds"].([]interface{})
					found := false
					for _, c := range cIds {
						if c == messageID {
							found = true
							break
						}
					}
					if !found {
						parent["childrenIds"] = append(cIds, messageID)
					}
				}
			}

			// If this is an assistant message or active branch leaf, update history.currentId
			if r, _ := updatedMsg["role"].(string); r == "assistant" {
				history["currentId"] = messageID
			}

			newChatJSON, err := json.Marshal(chatObj)
			if err != nil {
				return err
			}

			if createdAt == 0 {
				createdAt = now
			}
			fID := ""
			if folderID.Valid {
				fID = folderID.String
			}

			// Write to isolated chat database
			_, err = chatDB.Exec(`
				INSERT INTO chat_session (id, subdomain, user_id, title, folder_id, pinned, archived, chat_json, created_at, updated_at, last_read_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					chat_json = excluded.chat_json,
					updated_at = excluded.updated_at
			`, chatID, subdomain, userID, title, fID, pinned, archived, string(newChatJSON), createdAt, now, lastReadAt)
			if err != nil {
				return err
			}

			// Also append/update in messages DAG tree in isolated database
			role, _ := updatedMsg["role"].(string)
			contentStr := ""
			if c, ok := updatedMsg["content"].(string); ok {
				contentStr = c
			}
			parentID, _ := updatedMsg["parentId"].(string)
			metaBytes, _ := json.Marshal(updatedMsg)
			_, _ = chatDB.Exec(`
				INSERT INTO messages (id, parent_id, role, content, meta_json, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					content = CASE WHEN excluded.content != '' THEN excluded.content ELSE messages.content END,
					meta_json = excluded.meta_json,
					updated_at = excluded.updated_at
			`, messageID, parentID, role, contentStr, string(metaBytes), now, now)

			// Update lightweight index in central DB
			_, _ = d.conn.Exec(`
				UPDATE openwebui_chats SET updated_at = ? WHERE subdomain = ? AND user_id = ? AND id = ?
			`, now, subdomain, userID, chatID)

			return nil
		}
	}

	// Fallback legacy write if chatPool is somehow disabled
	var chatJSON string
	err := d.conn.QueryRow(`SELECT chat_json FROM openwebui_chats WHERE subdomain = ? AND user_id = ? AND id = ?`, subdomain, userID, chatID).Scan(&chatJSON)
	if err != nil {
		return err
	}
	var chatObj map[string]interface{}
	if err := json.Unmarshal([]byte(chatJSON), &chatObj); err != nil {
		return err
	}
	targetMap := chatObj
	if inner, ok := chatObj["chat"].(map[string]interface{}); ok && inner != nil {
		targetMap = inner
	}
	history, _ := targetMap["history"].(map[string]interface{})
	if history == nil {
		history = make(map[string]interface{})
		targetMap["history"] = history
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
	_, err := d.conn.Exec(`DELETE FROM openwebui_prompts WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

func (d *DB) GetOpenWebUIPromptByID(subdomain, id string) (*OpenWebUIPromptDB, error) {
	var p OpenWebUIPromptDB
	err := d.conn.QueryRow(`
		SELECT id, subdomain, user_id, command, name, content, created_at, updated_at
		FROM openwebui_prompts
		WHERE subdomain = ? AND id = ?
	`, subdomain, id).Scan(&p.ID, &p.Subdomain, &p.UserID, &p.Command, &p.Name, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) GetOpenWebUIPromptByCommand(subdomain, command string) (*OpenWebUIPromptDB, error) {
	var p OpenWebUIPromptDB
	err := d.conn.QueryRow(`
		SELECT id, subdomain, user_id, command, name, content, created_at, updated_at
		FROM openwebui_prompts
		WHERE subdomain = ? AND command = ?
	`, subdomain, command).Scan(&p.ID, &p.Subdomain, &p.UserID, &p.Command, &p.Name, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) DeleteOpenWebUIPromptByCommand(subdomain, userID, command string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_prompts WHERE subdomain = ? AND command = ?`, subdomain, command)
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
	StatusEmoji     string `json:"status_emoji"`
	StatusMessage   string `json:"status_message"`
	StatusExpiresAt int64  `json:"status_expires_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (d *DB) GetOpenWebUIUserProfile(subdomain, userID string) (*OpenWebUIUserProfileDB, error) {
	row := d.conn.QueryRow(`
		SELECT subdomain, user_id, name, profile_image_url, bio, gender, date_of_birth,
		       COALESCE(status_emoji, ''), COALESCE(status_message, ''), COALESCE(status_expires_at, 0), updated_at
		FROM openwebui_user_profiles
		WHERE subdomain = ? AND user_id = ?
	`, subdomain, userID)

	var p OpenWebUIUserProfileDB
	if err := row.Scan(&p.Subdomain, &p.UserID, &p.Name, &p.ProfileImageURL, &p.Bio, &p.Gender, &p.DateOfBirth, &p.StatusEmoji, &p.StatusMessage, &p.StatusExpiresAt, &p.UpdatedAt); err != nil {
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

func (d *DB) UpdateOpenWebUIUserStatus(subdomain, userID, emoji, message string, expiresAt int64) error {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_user_profiles (
			subdomain, user_id, status_emoji, status_message, status_expires_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(subdomain, user_id) DO UPDATE SET
			status_emoji = excluded.status_emoji,
			status_message = excluded.status_message,
			status_expires_at = excluded.status_expires_at,
			updated_at = excluded.updated_at
	`, subdomain, userID, emoji, message, expiresAt, now)
	return err
}

// ── Custom Models ─────────────────────────────────────────────────────────────

type OpenWebUICustomModelDB struct {
	ID               string `json:"id"`
	Subdomain        string `json:"subdomain"`
	UserID           string `json:"user_id"`
	Name             string `json:"name"`
	BaseModelID      string `json:"base_model_id"`
	MetaJSON         string `json:"meta_json"`
	ParamsJSON       string `json:"params_json"`
	AccessGrantsJSON string `json:"access_grants_json"`
	IsActive         bool   `json:"is_active"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (d *DB) GetOpenWebUICustomModels(subdomain string) ([]OpenWebUICustomModelDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, name, COALESCE(base_model_id, ''), meta_json, params_json, access_grants_json, is_active, created_at, updated_at
		FROM openwebui_custom_models
		WHERE subdomain = ?
		ORDER BY updated_at DESC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []OpenWebUICustomModelDB
	for rows.Next() {
		var m OpenWebUICustomModelDB
		var act int
		if err := rows.Scan(&m.ID, &m.Subdomain, &m.UserID, &m.Name, &m.BaseModelID, &m.MetaJSON, &m.ParamsJSON, &m.AccessGrantsJSON, &act, &m.CreatedAt, &m.UpdatedAt); err != nil {
			continue
		}
		m.IsActive = act == 1
		models = append(models, m)
	}
	if models == nil {
		models = []OpenWebUICustomModelDB{}
	}
	return models, nil
}

func (d *DB) GetOpenWebUICustomModelByID(subdomain, id string) (*OpenWebUICustomModelDB, error) {
	var m OpenWebUICustomModelDB
	var act int
	err := d.conn.QueryRow(`
		SELECT id, subdomain, user_id, name, COALESCE(base_model_id, ''), meta_json, params_json, access_grants_json, is_active, created_at, updated_at
		FROM openwebui_custom_models
		WHERE subdomain = ? AND id = ?
	`, subdomain, id).Scan(&m.ID, &m.Subdomain, &m.UserID, &m.Name, &m.BaseModelID, &m.MetaJSON, &m.ParamsJSON, &m.AccessGrantsJSON, &act, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.IsActive = act == 1
	return &m, nil
}

func (d *DB) UpsertOpenWebUICustomModel(m OpenWebUICustomModelDB) error {
	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	act := 0
	if m.IsActive {
		act = 1
	}
	if m.MetaJSON == "" {
		m.MetaJSON = "{}"
	}
	if m.ParamsJSON == "" {
		m.ParamsJSON = "{}"
	}
	if m.AccessGrantsJSON == "" {
		m.AccessGrantsJSON = "[]"
	}
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_custom_models (
			id, subdomain, user_id, name, base_model_id, meta_json, params_json, access_grants_json, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(subdomain, id) DO UPDATE SET
			name = excluded.name,
			base_model_id = excluded.base_model_id,
			meta_json = excluded.meta_json,
			params_json = excluded.params_json,
			access_grants_json = excluded.access_grants_json,
			is_active = excluded.is_active,
			updated_at = excluded.updated_at
	`, m.ID, m.Subdomain, m.UserID, m.Name, m.BaseModelID, m.MetaJSON, m.ParamsJSON, m.AccessGrantsJSON, act, m.CreatedAt, m.UpdatedAt)
	return err
}

func (d *DB) DeleteOpenWebUICustomModel(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_custom_models WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

func (d *DB) ToggleOpenWebUICustomModel(subdomain, id string) (*OpenWebUICustomModelDB, error) {
	now := time.Now().Unix()
	_, err := d.conn.Exec(`
		UPDATE openwebui_custom_models
		SET is_active = CASE WHEN is_active = 1 THEN 0 ELSE 1 END, updated_at = ?
		WHERE subdomain = ? AND id = ?
	`, now, subdomain, id)
	if err != nil {
		return nil, err
	}
	return d.GetOpenWebUICustomModelByID(subdomain, id)
}

// ── Knowledge Bases ───────────────────────────────────────────────────────────

type OpenWebUIKnowledgeDB struct {
	ID               string `json:"id"`
	Subdomain        string `json:"subdomain"`
	UserID           string `json:"user_id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	MetaJSON         string `json:"meta_json"`
	AccessGrantsJSON string `json:"access_grants_json"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (d *DB) GetOpenWebUIKnowledgeBases(subdomain, userID string) ([]OpenWebUIKnowledgeDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, name, description, meta_json, access_grants_json, created_at, updated_at
		FROM openwebui_knowledge
		WHERE subdomain = ?
		ORDER BY updated_at DESC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kbs []OpenWebUIKnowledgeDB
	for rows.Next() {
		var k OpenWebUIKnowledgeDB
		if err := rows.Scan(&k.ID, &k.Subdomain, &k.UserID, &k.Name, &k.Description, &k.MetaJSON, &k.AccessGrantsJSON, &k.CreatedAt, &k.UpdatedAt); err != nil {
			continue
		}
		kbs = append(kbs, k)
	}
	if kbs == nil {
		kbs = []OpenWebUIKnowledgeDB{}
	}
	return kbs, nil
}

func (d *DB) GetOpenWebUIKnowledgeByID(subdomain, id string) (*OpenWebUIKnowledgeDB, error) {
	var k OpenWebUIKnowledgeDB
	err := d.conn.QueryRow(`
		SELECT id, subdomain, user_id, name, description, meta_json, access_grants_json, created_at, updated_at
		FROM openwebui_knowledge
		WHERE subdomain = ? AND id = ?
	`, subdomain, id).Scan(&k.ID, &k.Subdomain, &k.UserID, &k.Name, &k.Description, &k.MetaJSON, &k.AccessGrantsJSON, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (d *DB) UpsertOpenWebUIKnowledge(k OpenWebUIKnowledgeDB) error {
	now := time.Now().Unix()
	if k.CreatedAt == 0 {
		k.CreatedAt = now
	}
	k.UpdatedAt = now
	if k.MetaJSON == "" {
		k.MetaJSON = "{}"
	}
	if k.AccessGrantsJSON == "" {
		k.AccessGrantsJSON = "[]"
	}
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_knowledge (
			id, subdomain, user_id, name, description, meta_json, access_grants_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			meta_json = excluded.meta_json,
			access_grants_json = excluded.access_grants_json,
			updated_at = excluded.updated_at
	`, k.ID, k.Subdomain, k.UserID, k.Name, k.Description, k.MetaJSON, k.AccessGrantsJSON, k.CreatedAt, k.UpdatedAt)
	return err
}

func (d *DB) DeleteOpenWebUIKnowledge(subdomain, userID, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_knowledge WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

type OpenWebUIToolServerDB struct {
	ID         string `json:"id"`
	Subdomain  string `json:"subdomain"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`      // "mcp" or "openapi"
	URL        string `json:"url"`       // e.g. "https://mcp-bridge.../mcp?room=..."
	AuthType   string `json:"auth_type"` // "none", "bearer"
	APIKey     string `json:"api_key"`
	ConfigJSON string `json:"config_json"`
	InfoJSON   string `json:"info_json"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

func (d *DB) GetOpenWebUIToolServers(subdomain string) ([]OpenWebUIToolServerDB, error) {
	rows, err := d.conn.Query(`
		SELECT id, subdomain, user_id, name, type, url, auth_type, api_key, config_json, info_json, created_at, updated_at
		FROM openwebui_tool_servers
		WHERE subdomain = ?
		ORDER BY created_at ASC
	`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []OpenWebUIToolServerDB
	for rows.Next() {
		var s OpenWebUIToolServerDB
		if err := rows.Scan(
			&s.ID, &s.Subdomain, &s.UserID, &s.Name, &s.Type, &s.URL,
			&s.AuthType, &s.APIKey, &s.ConfigJSON, &s.InfoJSON,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func (d *DB) UpsertOpenWebUIToolServer(s OpenWebUIToolServerDB) error {
	_, err := d.conn.Exec(`
		INSERT INTO openwebui_tool_servers (
			id, subdomain, user_id, name, type, url, auth_type, api_key, config_json, info_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(subdomain, id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			url = excluded.url,
			auth_type = excluded.auth_type,
			api_key = excluded.api_key,
			config_json = excluded.config_json,
			info_json = excluded.info_json,
			updated_at = excluded.updated_at
	`, s.ID, s.Subdomain, s.UserID, s.Name, s.Type, s.URL, s.AuthType, s.APIKey, s.ConfigJSON, s.InfoJSON, s.CreatedAt, s.UpdatedAt)
	return err
}

func (d *DB) DeleteOpenWebUIToolServer(subdomain, id string) error {
	_, err := d.conn.Exec(`DELETE FROM openwebui_tool_servers WHERE subdomain = ? AND id = ?`, subdomain, id)
	return err
}

