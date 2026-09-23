package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
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
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/chats")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" || path == "list":
		if r.Method == http.MethodGet {
			includePinned := r.URL.Query().Get("include_pinned") == "true"
			includeArchived := r.URL.Query().Get("include_archived") == "true"
			rawChats, err := h.db.GetOpenWebUIChats(subdomain, includeArchived, includePinned)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			var list []OpenWebUIChatTitleIdResponse
			for _, rc := range rawChats {
				list = append(list, OpenWebUIChatTitleIdResponse{
					ID:        rc.ID,
					Title:     rc.Title,
					CreatedAt: rc.CreatedAt,
					UpdatedAt: rc.UpdatedAt,
					Archived:  rc.Archived,
					Pinned:    rc.Pinned,
				})
			}
			if list == nil {
				list = []OpenWebUIChatTitleIdResponse{}
			}
			writeJSON(w, http.StatusOK, list)
			return
		}
		if r.Method == http.MethodDelete {
			_ = h.db.DeleteAllOpenWebUIChats(subdomain)
			writeJSON(w, http.StatusOK, true)
			return
		}

	case path == "pinned":
		rawChats, err := h.db.GetOpenWebUIPinnedChats(subdomain)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		var list []OpenWebUIChatTitleIdResponse
		for _, rc := range rawChats {
			list = append(list, OpenWebUIChatTitleIdResponse{
				ID:        rc.ID,
				Title:     rc.Title,
				CreatedAt: rc.CreatedAt,
				UpdatedAt: rc.UpdatedAt,
				Archived:  rc.Archived,
				Pinned:    true,
			})
		}
		if list == nil {
			list = []OpenWebUIChatTitleIdResponse{}
		}
		writeJSON(w, http.StatusOK, list)
		return

	case path == "all/archived" || path == "archived":
		rawChats, err := h.db.GetOpenWebUIArchivedChats(subdomain)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		var list []OpenWebUIChatTitleIdResponse
		for _, rc := range rawChats {
			list = append(list, OpenWebUIChatTitleIdResponse{
				ID:        rc.ID,
				Title:     rc.Title,
				CreatedAt: rc.CreatedAt,
				UpdatedAt: rc.UpdatedAt,
				Archived:  true,
				Pinned:    rc.Pinned,
			})
		}
		if list == nil {
			list = []OpenWebUIChatTitleIdResponse{}
		}
		writeJSON(w, http.StatusOK, list)
		return

	case path == "archived/count":
		rawChats, _ := h.db.GetOpenWebUIArchivedChats(subdomain)
		writeJSON(w, http.StatusOK, len(rawChats))
		return

	case path == "all/tags" || path == "tags" || strings.HasSuffix(path, "/tags"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case strings.HasPrefix(path, "folder/"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case path == "read":
		writeJSON(w, http.StatusOK, map[string]any{"updated_count": 0, "folder_unread_counts": map[string]int{}})
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

			now := time.Now().Unix()
			_ = h.db.UpsertOpenWebUIChat(subdomain, id, title, string(body), folderID)

			writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
				ID:        id,
				UserID:    "admin",
				Title:     title,
				Chat:      parsed["chat"],
				CreatedAt: now,
				UpdatedAt: now,
				Archived:  false,
				Pinned:    false,
				FolderID:  optStr(folderID),
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
				title, chatJSON, pinned, archived, folderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				newPinned := !pinned
				_ = h.db.SetOpenWebUIChatPinned(subdomain, chatID, newPinned)
				var chatObj any
				_ = json.Unmarshal([]byte(chatJSON), &chatObj)
				writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
					ID:        chatID,
					UserID:    "admin",
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
				_, _, pinned, _, _, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
				if err != nil {
					writeJSON(w, http.StatusOK, false)
					return
				}
				writeJSON(w, http.StatusOK, pinned)
				return

			case "archive":
				title, chatJSON, pinned, archived, folderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				newArchived := !archived
				_ = h.db.SetOpenWebUIChatArchived(subdomain, chatID, newArchived)
				var chatObj any
				_ = json.Unmarshal([]byte(chatJSON), &chatObj)
				writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
					ID:        chatID,
					UserID:    "admin",
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
				var form map[string]string
				_ = json.Unmarshal(body, &form)
				folderID := form["folder_id"]
				_ = h.db.SetOpenWebUIChatFolder(subdomain, chatID, folderID)
				writeJSON(w, http.StatusOK, true)
				return

			case "clone":
				title, chatJSON, _, _, folderID, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "chat not found")
					return
				}
				randBytes := make([]byte, 16)
				_, _ = rand.Read(randBytes)
				newID := hex.EncodeToString(randBytes)
				now := time.Now().Unix()
				_ = h.db.UpsertOpenWebUIChat(subdomain, newID, title+" (Copy)", chatJSON, folderID)
				var chatObj any
				_ = json.Unmarshal([]byte(chatJSON), &chatObj)
				writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
					ID:        newID,
					UserID:    "admin",
					Title:     title + " (Copy)",
					Chat:      chatObj,
					CreatedAt: now,
					UpdatedAt: now,
					Pinned:    false,
					Archived:  false,
				})
				return

			default:
				if strings.HasPrefix(subAction, "messages/") || subAction == "unread" {
					writeJSON(w, http.StatusOK, true)
					return
				}
			}
		}

		if r.Method == http.MethodGet {
			title, chatJSON, pinned, archived, folderID, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
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

			writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
				ID:        chatID,
				UserID:    "admin",
				Title:     title,
				Chat:      chatObj,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				Archived:  archived,
				Pinned:    pinned,
				FolderID:  optStr(folderID),
			})
			return
		}

		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]interface{}
			_ = json.Unmarshal(body, &parsed)

			title := "Chat"
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

			_ = h.db.UpsertOpenWebUIChat(subdomain, chatID, title, string(body), folderID)
			now := time.Now().Unix()
			writeJSON(w, http.StatusOK, OpenWebUIChatResponse{
				ID:        chatID,
				UserID:    "admin",
				Title:     title,
				Chat:      parsed["chat"],
				CreatedAt: now,
				UpdatedAt: now,
				Archived:  false,
				Pinned:    false,
				FolderID:  optStr(folderID),
			})
			return
		}

		if r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIChat(subdomain, chatID)
			writeJSON(w, http.StatusOK, true)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}
