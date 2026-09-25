package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// handleOpenWebUIChats handles GET/POST/DELETE on /api/v1/chats/*
func (h *APIHandler) handleOpenWebUIChats(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/chats")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" || path == "list":
		if r.Method == http.MethodGet {
			includePinned := r.URL.Query().Get("include_pinned") == "true"
			includeArchived := r.URL.Query().Get("include_archived") == "true"
			includeFolders := r.URL.Query().Get("include_folders") == "true"
			rawChats, err := h.db.GetOpenWebUIChats(subdomain, u.ID, includeArchived, includePinned, includeFolders)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if pageStr := r.URL.Query().Get("page"); pageStr != "" {
				if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
					limit := 60
					skip := (page - 1) * limit
					if skip >= len(rawChats) {
						rawChats = nil
					} else {
						end := skip + limit
						if end > len(rawChats) {
							end = len(rawChats)
						}
						rawChats = rawChats[skip:end]
					}
				}
			}
			var list []OpenWebUIChatTitleIdResponse
			for _, rc := range rawChats {
				var lastRead *int64
				if rc.LastReadAt > 0 {
					lr := rc.LastReadAt
					lastRead = &lr
				}
				list = append(list, OpenWebUIChatTitleIdResponse{
					ID:         rc.ID,
					Title:      rc.Title,
					CreatedAt:  rc.CreatedAt,
					UpdatedAt:  rc.UpdatedAt,
					LastReadAt: lastRead,
					Archived:   rc.Archived,
					Pinned:     rc.Pinned,
					FolderID:   optStr(rc.FolderID),
				})
			}
			if list == nil {
				list = []OpenWebUIChatTitleIdResponse{}
			}
			writeJSON(w, http.StatusOK, list)
			return
		}
		if r.Method == http.MethodDelete {
			_ = h.db.DeleteAllOpenWebUIChats(subdomain, u.ID)
			writeJSON(w, http.StatusOK, true)
			return
		}

	case path == "pinned":
		rawChats, err := h.db.GetOpenWebUIPinnedChats(subdomain, u.ID)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		var list []OpenWebUIChatTitleIdResponse
		for _, rc := range rawChats {
			var lastRead *int64
			if rc.LastReadAt > 0 {
				lr := rc.LastReadAt
				lastRead = &lr
			}
			list = append(list, OpenWebUIChatTitleIdResponse{
				ID:         rc.ID,
				Title:      rc.Title,
				CreatedAt:  rc.CreatedAt,
				UpdatedAt:  rc.UpdatedAt,
				LastReadAt: lastRead,
				Archived:   rc.Archived,
				Pinned:     true,
				FolderID:   optStr(rc.FolderID),
			})
		}
		if list == nil {
			list = []OpenWebUIChatTitleIdResponse{}
		}
		writeJSON(w, http.StatusOK, list)
		return

	case path == "all/archived" || path == "archived":
		rawChats, err := h.db.GetOpenWebUIArchivedChats(subdomain, u.ID)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		var list []OpenWebUIChatTitleIdResponse
		for _, rc := range rawChats {
			var lastRead *int64
			if rc.LastReadAt > 0 {
				lr := rc.LastReadAt
				lastRead = &lr
			}
			list = append(list, OpenWebUIChatTitleIdResponse{
				ID:         rc.ID,
				Title:      rc.Title,
				CreatedAt:  rc.CreatedAt,
				UpdatedAt:  rc.UpdatedAt,
				LastReadAt: lastRead,
				Archived:   true,
				Pinned:     rc.Pinned,
				FolderID:   optStr(rc.FolderID),
			})
		}
		if list == nil {
			list = []OpenWebUIChatTitleIdResponse{}
		}
		writeJSON(w, http.StatusOK, list)
		return

	case path == "archived/count":
		rawChats, _ := h.db.GetOpenWebUIArchivedChats(subdomain, u.ID)
		writeJSON(w, http.StatusOK, len(rawChats))
		return

	case path == "all/tags" || path == "tags" || strings.HasSuffix(path, "/tags"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case strings.HasPrefix(path, "folder/"):
		folderID := strings.TrimPrefix(path, "folder/")
		folderID = strings.TrimSuffix(folderID, "/list")
		folderID = strings.TrimSuffix(folderID, "/")

		rawChats, err := h.db.GetOpenWebUIChatsByFolder(subdomain, u.ID, folderID)
		if err != nil {
			writeJSON(w, http.StatusOK, []OpenWebUIChatTitleIdResponse{})
			return
		}
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
				limit := 10
				skip := (page - 1) * limit
				if skip >= len(rawChats) {
					rawChats = nil
				} else {
					end := skip + limit
					if end > len(rawChats) {
						end = len(rawChats)
					}
					rawChats = rawChats[skip:end]
				}
			}
		}
		var list []OpenWebUIChatTitleIdResponse
		for _, rc := range rawChats {
			var lastRead *int64
			if rc.LastReadAt > 0 {
				lr := rc.LastReadAt
				lastRead = &lr
			}
			list = append(list, OpenWebUIChatTitleIdResponse{
				ID:         rc.ID,
				Title:      rc.Title,
				CreatedAt:  rc.CreatedAt,
				UpdatedAt:  rc.UpdatedAt,
				LastReadAt: lastRead,
				Archived:   rc.Archived,
				Pinned:     rc.Pinned,
				FolderID:   optStr(rc.FolderID),
			})
		}
		if list == nil {
			list = []OpenWebUIChatTitleIdResponse{}
		}
		writeJSON(w, http.StatusOK, list)
		return

	case path == "read":
		if r.Method == http.MethodPost {
			count, _ := h.db.MarkAllOpenWebUIChatsRead(subdomain, u.ID)
			unreadCounts := h.getFolderUnreadCounts(subdomain, u.ID)
			writeJSON(w, http.StatusOK, map[string]any{
				"updated_count":        count,
				"folder_unread_counts": unreadCounts,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"updated_count":        0,
			"folder_unread_counts": h.getFolderUnreadCounts(subdomain, u.ID),
		})
		return

	case path == "config":
		writeJSON(w, http.StatusOK, OpenWebUIChatConfigForm{
			ContextCompactionModel:               "",
			EnableContextCompaction:              false,
			ContextCompactionTokenThreshold:      50000,
			ContextCompactionTokenCap:            80000,
			ContextCompactionRetentionPercentage: 40,
			ContextCompactionPromptTemplate:      "",
			EnableToolPermissions:                false,
		})
		return

	case path == "new":
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]interface{}
			_ = json.Unmarshal(body, &parsed)

			id := ""
			if c, ok := parsed["chat"].(map[string]interface{}); ok {
				if idVal, ok := c["id"].(string); ok && idVal != "" {
					id = idVal
				}
			}
			if id == "" {
				randBytes := make([]byte, 16)
				_, _ = rand.Read(randBytes)
				id = hex.EncodeToString(randBytes)
			}

			title := "New Chat"
			if t, ok := parsed["title"].(string); ok && t != "" {
				title = t
			} else if c, ok := parsed["chat"].(map[string]interface{}); ok {
				if t, ok := c["title"].(string); ok && t != "" {
					title = t
				}
			}

			folderID := ""
			if fid, ok := parsed["folder_id"].(string); ok {
				folderID = fid
			}

			mergedObj, err := h.db.MergeUpdateOpenWebUIChat(subdomain, u.ID, id, parsed, folderID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}

			title, chatJSON, pinned, archived, updatedFolderID, createdAt, updatedAt, _ := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, id)
			var chatData map[string]interface{}
			_ = json.Unmarshal([]byte(chatJSON), &chatData)
			chatObj := chatData["chat"]
			if chatObj == nil {
				chatObj = chatData
			}
			var variables map[string]any
			if v, ok := chatData["variables"].(map[string]any); ok {
				variables = v
			} else if v, ok := mergedObj["variables"].(map[string]any); ok {
				variables = v
			}

			writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
				ID:        id,
				UserID:    u.ID,
				Title:     title,
				Chat:      chatObj,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				Archived:  archived,
				Pinned:    pinned,
				FolderID:  optStr(updatedFolderID),
				Variables: variables,
			})
			return
		}

	default: // /api/v1/chats/{id} or /api/v1/chats/{id}/...
		chatID := path
		subAction := ""
		if slashIdx := strings.Index(path, "/"); slashIdx > 0 {
			chatID = path[:slashIdx]
			subAction = path[slashIdx+1:]
		}

		if subAction != "" {
			switch subAction {
			case "pin":
				// Toggle pin status
				title, chatJSON, pinned, archived, folderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				newPinned := !pinned
				_ = h.db.SetOpenWebUIChatPinned(subdomain, u.ID, chatID, newPinned)
				var chatObj any
				_ = json.Unmarshal([]byte(chatJSON), &chatObj)
				writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
					ID:        chatID,
					UserID:    u.ID,
					Title:     title,
					Chat:      chatObj,
					CreatedAt: createdAt,
					UpdatedAt: updatedAt,
					Pinned:    newPinned,
					Archived:  archived,
					FolderID:  optStr(folderID),
				})
				return

			case "pinned":
				_, _, pinned, _, _, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
				if err != nil {
					writeJSON(w, http.StatusOK, false)
					return
				}
				writeJSON(w, http.StatusOK, pinned)
				return

			case "archive":
				title, chatJSON, pinned, archived, folderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				newArchived := !archived
				_ = h.db.SetOpenWebUIChatArchived(subdomain, u.ID, chatID, newArchived)
				var chatObj any
				_ = json.Unmarshal([]byte(chatJSON), &chatObj)
				writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
					ID:        chatID,
					UserID:    u.ID,
					Title:     title,
					Chat:      chatObj,
					CreatedAt: createdAt,
					UpdatedAt: updatedAt,
					Pinned:    pinned,
					Archived:  newArchived,
					FolderID:  optStr(folderID),
				})
				return

			case "folder":
				body, _ := io.ReadAll(r.Body)
				var form map[string]any
				_ = json.Unmarshal(body, &form)
				folderID, _ := form["folder_id"].(string)
				_ = h.db.SetOpenWebUIChatFolder(subdomain, u.ID, chatID, folderID)

				title, chatJSON, pinned, archived, updatedFolderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				var chatData map[string]interface{}
				_ = json.Unmarshal([]byte(chatJSON), &chatData)
				chatObj := chatData["chat"]
				if chatObj == nil {
					chatObj = chatData
				}
				var variables map[string]any
				if v, ok := chatData["variables"].(map[string]any); ok {
					variables = v
				}

				resp := OpenWebUIChatResponse{
					ID:        chatID,
					UserID:    u.ID,
					Title:     title,
					Chat:      chatObj,
					CreatedAt: createdAt,
					UpdatedAt: updatedAt,
					Pinned:    pinned,
					Archived:  archived,
					FolderID:  optStr(updatedFolderID),
					Variables: variables,
				}

				payload, _ := json.Marshal(map[string]any{
					"folder_id": optStr(updatedFolderID),
				})
				globalSocketHub.broadcastToRoom("user:"+u.ID, []byte(fmt.Sprintf(`42["events",{"type":"chat:folder:updated","data":%s}]`, string(payload))), "")
				globalSocketHub.broadcastToRoom("user:"+u.ID, []byte(`42["events",{"type":"chat:list"}]`), "")

				writeJSON(w, http.StatusOK, resp)
				return

			case "clone":
				title, chatJSON, _, _, folderID, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				randBytes := make([]byte, 16)
				_, _ = rand.Read(randBytes)
				newID := hex.EncodeToString(randBytes)
				now := time.Now().Unix()
				_ = h.db.UpsertOpenWebUIChat(subdomain, u.ID, newID, title+" (Copy)", chatJSON, folderID)
				var chatObj any
				_ = json.Unmarshal([]byte(chatJSON), &chatObj)
				writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
					ID:        newID,
					UserID:    u.ID,
					Title:     title + " (Copy)",
					Chat:      chatObj,
					CreatedAt: now,
					UpdatedAt: now,
					Pinned:    false,
					Archived:  false,
				})
				return

			case "unread":
				_ = h.db.MarkOpenWebUIChatUnread(subdomain, u.ID, chatID)
				unreadCounts := h.getFolderUnreadCounts(subdomain, u.ID)
				writeJSON(w, http.StatusOK, map[string]any{
					"chat_id":              chatID,
					"last_read_at":         0,
					"folder_unread_counts": unreadCounts,
				})
				return

			default:
				if strings.HasPrefix(subAction, "messages/") {
					writeJSON(w, http.StatusOK, true)
					return
				}
			}
		}

		if r.Method == http.MethodGet {
			title, chatJSON, pinned, archived, folderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
			if err != nil {
				writeJSONError(w, http.StatusNotFound, "chat not found")
				return
			}
			var chatData map[string]interface{}
			_ = json.Unmarshal([]byte(chatJSON), &chatData)
			chatObj := chatData["chat"]
			if chatObj == nil {
				chatObj = chatData
			}
			var variables map[string]any
			if v, ok := chatData["variables"].(map[string]any); ok {
				variables = v
			}

			writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
				ID:        chatID,
				UserID:    u.ID,
				Title:     title,
				Chat:      chatObj,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				Archived:  archived,
				Pinned:    pinned,
				FolderID:  optStr(folderID),
				Variables: variables,
			})
			return
		}

		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]interface{}
			_ = json.Unmarshal(body, &parsed)

			folderID := ""
			if fid, ok := parsed["folder_id"].(string); ok {
				folderID = fid
			}

			mergedObj, err := h.db.MergeUpdateOpenWebUIChat(subdomain, u.ID, chatID, parsed, folderID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}

			title, chatJSON, pinned, archived, updatedFolderID, createdAt, updatedAt, _ := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
			var chatData map[string]interface{}
			_ = json.Unmarshal([]byte(chatJSON), &chatData)
			chatObj := chatData["chat"]
			if chatObj == nil {
				chatObj = chatData
			}
			var variables map[string]any
			if v, ok := chatData["variables"].(map[string]any); ok {
				variables = v
			} else if v, ok := mergedObj["variables"].(map[string]any); ok {
				variables = v
			}

			writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
				ID:        chatID,
				UserID:    u.ID,
				Title:     title,
				Chat:      chatObj,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				Archived:  archived,
				Pinned:    pinned,
				FolderID:  optStr(updatedFolderID),
				Variables: variables,
			})
			return
		}

		if r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIChat(subdomain, u.ID, chatID)
			writeJSON(w, http.StatusOK, true)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}
