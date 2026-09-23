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
			rawChats, err := h.db.GetOpenWebUIChats(subdomain)
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
					Archived:  false,
					Pinned:    false,
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

	case path == "pinned" || strings.HasSuffix(path, "/pinned"):
		if r.Method == http.MethodGet {
			if path == "pinned" {
				writeJSON(w, http.StatusOK, []any{})
			} else {
				writeJSON(w, http.StatusOK, false)
			}
			return
		}
		if r.Method == http.MethodPost {
			writeJSON(w, http.StatusOK, map[string]bool{"pinned": true})
			return
		}

	case strings.HasSuffix(path, "/pin"):
		writeJSON(w, http.StatusOK, map[string]bool{"pinned": true})
		return

	case path == "all/tags" || path == "tags" || strings.HasSuffix(path, "/tags"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case path == "all/archived" || path == "archived" || strings.HasPrefix(path, "folder/"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case path == "archived/count":
		writeJSON(w, http.StatusOK, 0)
		return

	case path == "read":
		writeJSON(w, http.StatusOK, true)
		return

	case path == "config":
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"CONTEXT_COMPACTION_TOKEN_THRESHOLD": 50000,
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

			now := time.Now().Unix()
			_ = h.db.UpsertOpenWebUIChat(subdomain, id, title, string(body))

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":         id,
				"user_id":    "admin",
				"title":      title,
				"chat":       parsed["chat"],
				"created_at": now,
				"updated_at": now,
				"archived":   false,
				"pinned":     false,
			})
			return
		}

	default: // /api/v1/chats/{id} or /api/v1/chats/{id}/...
		chatID := path
		if slashIdx := strings.Index(path, "/"); slashIdx > 0 {
			chatID = path[:slashIdx]
			subAction := path[slashIdx+1:]
			if strings.HasPrefix(subAction, "messages/") || subAction == "unread" {
				writeJSON(w, http.StatusOK, true)
				return
			}
		}

		if r.Method == http.MethodGet {
			title, chatJSON, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
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

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":         chatID,
				"user_id":    "admin",
				"title":      title,
				"chat":       chatObj,
				"created_at": createdAt,
				"updated_at": updatedAt,
				"archived":   false,
				"pinned":     false,
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

			_ = h.db.UpsertOpenWebUIChat(subdomain, chatID, title, string(body))
			now := time.Now().Unix()
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":         chatID,
				"user_id":    "admin",
				"title":      title,
				"chat":       parsed["chat"],
				"created_at": now,
				"updated_at": now,
				"archived":   false,
				"pinned":     false,
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
