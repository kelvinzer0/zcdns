package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"zcdns-backend/db"
)

func promptToModel(p *db.OpenWebUIPromptDB, u *OpenWebUISessionUserInfoResponse) map[string]any {
	if p == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":            p.ID,
		"command":       p.Command,
		"name":          p.Name,
		"content":       p.Content,
		"user_id":       p.UserID,
		"created_at":    p.CreatedAt,
		"updated_at":    p.UpdatedAt,
		"data":          map[string]any{},
		"meta":          map[string]any{},
		"tags":          []string{},
		"is_active":     true,
		"version_id":    nil,
		"access_grants": []any{},
		"write_access":  true,
		"user": map[string]any{
			"id":    u.ID,
			"name":  u.Name,
			"email": u.Email,
		},
	}
}

// handleOpenWebUIPrompts handles /api/v1/prompts/* with DB backing
func (h *APIHandler) handleOpenWebUIPrompts(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts")
	path = strings.TrimPrefix(path, "/")

	if path == "list" {
		prompts, _ := h.db.GetOpenWebUIPrompts(subdomain, u.ID)
		query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
		var items []any
		for _, p := range prompts {
			if query != "" {
				lCmd := strings.ToLower(p.Command)
				lName := strings.ToLower(p.Name)
				lContent := strings.ToLower(p.Content)
				if !strings.Contains(lCmd, query) && !strings.Contains(lName, query) && !strings.Contains(lContent, query) {
					continue
				}
			}
			items = append(items, promptToModel(&p, u))
		}
		if items == nil {
			items = []any{}
		}
		total := len(items)
		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		limit := 30
		skip := (page - 1) * limit
		if skip >= total {
			items = []any{}
		} else {
			end := skip + limit
			if end > total {
				end = total
			}
			items = items[skip:end]
		}
		writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
			Items: items,
			Total: int64(total),
		})
		return
	}

	if path == "create" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		var form struct {
			ID      string `json:"id"`
			Command string `json:"command"`
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		_ = json.Unmarshal(body, &form)
		pID := form.ID
		if pID == "" {
			pID = uuid.New().String()
		}
		cmd := strings.TrimPrefix(form.Command, "/")
		_ = h.db.UpsertOpenWebUIPrompt(subdomain, u.ID, pID, cmd, form.Name, form.Content)
		p, err := h.db.GetOpenWebUIPromptByID(subdomain, pID)
		if err != nil || p == nil {
			p = &db.OpenWebUIPromptDB{
				ID:        pID,
				Subdomain: subdomain,
				UserID:    u.ID,
				Command:   cmd,
				Name:      form.Name,
				Content:   form.Content,
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
			}
		}
		writeJSON(w, http.StatusOK, promptToModel(p, u))
		return
	}

	if path == "tags" {
		writeJSON(w, http.StatusOK, []string{})
		return
	}

	// Sub-actions on id/:prompt_id/*
	if strings.HasPrefix(path, "id/") {
		sub := strings.TrimPrefix(path, "id/")
		parts := strings.Split(sub, "/")
		promptID := parts[0]

		if len(parts) == 1 {
			if r.Method == http.MethodGet {
				p, err := h.db.GetOpenWebUIPromptByID(subdomain, promptID)
				if err != nil || p == nil {
					writeJSONError(w, http.StatusNotFound, "Prompt not found")
					return
				}
				writeJSON(w, http.StatusOK, promptToModel(p, u))
				return
			}
			if r.Method == http.MethodDelete {
				_ = h.db.DeleteOpenWebUIPrompt(subdomain, u.ID, promptID)
				writeJSON(w, http.StatusOK, true)
				return
			}
		}

		if len(parts) >= 2 {
			action := parts[1]
			if action == "delete" {
				_ = h.db.DeleteOpenWebUIPrompt(subdomain, u.ID, promptID)
				writeJSON(w, http.StatusOK, true)
				return
			}
			if action == "update" {
				if len(parts) >= 3 && parts[2] == "meta" {
					body, _ := io.ReadAll(r.Body)
					var form struct {
						Name    string   `json:"name"`
						Command string   `json:"command"`
						Tags    []string `json:"tags"`
					}
					_ = json.Unmarshal(body, &form)
					cmd := strings.TrimPrefix(form.Command, "/")
					p, _ := h.db.GetOpenWebUIPromptByID(subdomain, promptID)
					content := ""
					name := form.Name
					if p != nil {
						content = p.Content
						if name == "" {
							name = p.Name
						}
						if cmd == "" {
							cmd = p.Command
						}
					}
					_ = h.db.UpsertOpenWebUIPrompt(subdomain, u.ID, promptID, cmd, name, content)
					pUpdated, _ := h.db.GetOpenWebUIPromptByID(subdomain, promptID)
					writeJSON(w, http.StatusOK, promptToModel(pUpdated, u))
					return
				}
				if len(parts) >= 3 && parts[2] == "version" {
					p, _ := h.db.GetOpenWebUIPromptByID(subdomain, promptID)
					writeJSON(w, http.StatusOK, promptToModel(p, u))
					return
				}
				body, _ := io.ReadAll(r.Body)
				var form struct {
					Command string `json:"command"`
					Name    string `json:"name"`
					Content string `json:"content"`
				}
				_ = json.Unmarshal(body, &form)
				cmd := strings.TrimPrefix(form.Command, "/")
				_ = h.db.UpsertOpenWebUIPrompt(subdomain, u.ID, promptID, cmd, form.Name, form.Content)
				pUpdated, _ := h.db.GetOpenWebUIPromptByID(subdomain, promptID)
				writeJSON(w, http.StatusOK, promptToModel(pUpdated, u))
				return
			}
			if action == "toggle" || action == "access" {
				p, _ := h.db.GetOpenWebUIPromptByID(subdomain, promptID)
				writeJSON(w, http.StatusOK, promptToModel(p, u))
				return
			}
			if action == "history" {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
		}
	}

	// Sub-actions on command/:command/*
	if strings.HasPrefix(path, "command/") {
		cmd := strings.TrimPrefix(path, "command/")
		cmd = strings.TrimPrefix(cmd, "/")
		if strings.HasSuffix(cmd, "/delete") {
			cmd = strings.TrimSuffix(cmd, "/delete")
			cmd = strings.TrimPrefix(cmd, "/")
			_ = h.db.DeleteOpenWebUIPromptByCommand(subdomain, u.ID, cmd)
			writeJSON(w, http.StatusOK, true)
			return
		}
		p, err := h.db.GetOpenWebUIPromptByCommand(subdomain, cmd)
		if err != nil || p == nil {
			writeJSONError(w, http.StatusNotFound, "Prompt not found")
			return
		}
		writeJSON(w, http.StatusOK, promptToModel(p, u))
		return
	}

	prompts, _ := h.db.GetOpenWebUIPrompts(subdomain, u.ID)
	var list []any
	for _, p := range prompts {
		list = append(list, promptToModel(&p, u))
	}
	if list == nil {
		list = []any{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleOpenWebUIFolders handles /api/v1/folders/* with DB backing
func (h *APIHandler) handleOpenWebUIFolders(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/folders")
	path = strings.TrimPrefix(path, "/")

	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			rawFolders, err := h.db.GetOpenWebUIFolders(subdomain, u.ID)
			if err != nil {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
			unreadCounts := h.getFolderUnreadCounts(subdomain, u.ID)
			var list []OpenWebUIFolderNameIdResponse
			for _, rf := range rawFolders {
				var parent *string
				if rf.ParentID != "" {
					p := rf.ParentID
					parent = &p
				}
				list = append(list, OpenWebUIFolderNameIdResponse{
					ID:          rf.ID,
					Name:        rf.Name,
					ParentID:    parent,
					IsExpanded:  rf.IsExpanded,
					UnreadCount: int64(unreadCounts[rf.ID]),
					CreatedAt:   rf.CreatedAt,
					UpdatedAt:   rf.UpdatedAt,
				})
			}
			if list == nil {
				list = []OpenWebUIFolderNameIdResponse{}
			}
			writeJSON(w, http.StatusOK, list)
			return
		}

		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var form map[string]interface{}
			_ = json.Unmarshal(body, &form)

			name := "New Folder"
			if n, ok := form["name"].(string); ok && n != "" {
				name = n
			}
			parentID := ""
			if p, ok := form["parent_id"].(string); ok {
				parentID = p
			}

			randBytes := make([]byte, 8)
			_, _ = rand.Read(randBytes)
			folderID := hex.EncodeToString(randBytes)
			_ = h.db.CreateOpenWebUIFolder(subdomain, u.ID, folderID, name, parentID)

			now := time.Now().Unix()
			var parent *string
			if parentID != "" {
				parent = &parentID
			}
			writeJSON(w, http.StatusOK, OpenWebUIFolderNameIdResponse{
				ID:          folderID,
				Name:        name,
				ParentID:    parent,
				IsExpanded:  false,
				UnreadCount: 0,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			return
		}
	}

	// Shared folders endpoint (/api/v1/folders/shared)
	if path == "shared" || path == "shared/" {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
	}

	// Sub-actions on /folders/{id}/*
	if path != "" {
		parts := strings.Split(path, "/")
		folderID := parts[0]

		if len(parts) == 1 {
			if r.Method == http.MethodGet {
				folder, err := h.db.GetOpenWebUIFolderByID(subdomain, u.ID, folderID)
				if err != nil {
					writeJSONError(w, http.StatusNotFound, "Folder not found")
					return
				}
				unreadCounts := h.getFolderUnreadCounts(subdomain, u.ID)
				var parent *string
				if folder.ParentID != "" {
					p := folder.ParentID
					parent = &p
				}
				writeJSON(w, http.StatusOK, OpenWebUIFolderNameIdResponse{
					ID:          folder.ID,
					Name:        folder.Name,
					ParentID:    parent,
					IsExpanded:  folder.IsExpanded,
					UnreadCount: int64(unreadCounts[folder.ID]),
					CreatedAt:   folder.CreatedAt,
					UpdatedAt:   folder.UpdatedAt,
				})
				return
			}
			if r.Method == http.MethodDelete {
				_ = h.db.DeleteOpenWebUIFolder(subdomain, u.ID, folderID)
				writeJSON(w, http.StatusOK, true)
				return
			}
		}

		if len(parts) >= 2 {
			action := parts[1]
			subAction := ""
			if len(parts) >= 3 {
				subAction = parts[2]
			}

			if action == "shared" {
				if r.Method == http.MethodGet {
					if subAction == "chats" || subAction == "" {
						rawChats, err := h.db.GetOpenWebUIChatsByFolder(subdomain, u.ID, folderID)
						if err != nil {
							rawChats = nil
						}
						total := len(rawChats)
						hasMore := false
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
									hasMore = end < total
									rawChats = rawChats[skip:end]
								}
							}
						}
						var chatItems []map[string]any
						for _, c := range rawChats {
							var lastRead *int64
							if c.LastReadAt > 0 {
								lr := c.LastReadAt
								lastRead = &lr
							}
							chatItems = append(chatItems, map[string]any{
								"id":           c.ID,
								"title":        c.Title,
								"user_id":      u.ID,
								"created_at":   c.CreatedAt,
								"updated_at":   c.UpdatedAt,
								"last_read_at": lastRead,
								"folder_id":    c.FolderID,
								"owner_name":   u.Name,
								"active":       false,
								"readonly":     false,
							})
						}
						if chatItems == nil {
							chatItems = []map[string]any{}
						}
						writeJSON(w, http.StatusOK, map[string]any{
							"chats":             chatItems,
							"folder_permission": "write",
							"total":             total,
							"has_more":          hasMore,
						})
						return
					}
					writeJSON(w, http.StatusOK, []any{})
					return
				}
			}

			if action == "read" {
				folderIDs, _ := h.db.GetOpenWebUISubtreeFolderIDs(subdomain, u.ID, folderID)
				updatedCount, _ := h.db.MarkOpenWebUIChatsReadByFolderIDs(subdomain, u.ID, folderIDs)
				unreadCounts := h.getFolderUnreadCounts(subdomain, u.ID)

				payload, _ := json.Marshal(map[string]interface{}{
					"folder_id":            folderID,
					"folder_ids":           folderIDs,
					"updated_count":        updatedCount,
					"folder_unread_counts": unreadCounts,
				})
				globalSocketHub.broadcastToRoom("user:"+u.ID, []byte(fmt.Sprintf(`42["events",{"type":"chat:list","data":%s}]`, string(payload))), "")

				writeJSON(w, http.StatusOK, map[string]interface{}{
					"folder_id":            folderID,
					"folder_ids":           folderIDs,
					"updated_count":        updatedCount,
					"folder_unread_counts": unreadCounts,
				})
				return
			}

			if action == "update" {
				body, _ := io.ReadAll(r.Body)
				if subAction == "parent" {
					var form map[string]string
					_ = json.Unmarshal(body, &form)
					parentID := form["parent_id"]
					_ = h.db.UpdateOpenWebUIFolderParent(subdomain, u.ID, folderID, parentID)
					writeJSON(w, http.StatusOK, true)
					return
				}
				if subAction == "expanded" {
					var form map[string]bool
					_ = json.Unmarshal(body, &form)
					_ = h.db.UpdateOpenWebUIFolderExpanded(subdomain, u.ID, folderID, form["is_expanded"])
					writeJSON(w, http.StatusOK, true)
					return
				}
				// Default update: name
				var form map[string]string
				_ = json.Unmarshal(body, &form)
				if name, ok := form["name"]; ok && name != "" {
					_ = h.db.UpdateOpenWebUIFolderName(subdomain, u.ID, folderID, name)
				}
				writeJSON(w, http.StatusOK, true)
				return
			}

			if action == "expanded" {
				body, _ := io.ReadAll(r.Body)
				var form map[string]bool
				_ = json.Unmarshal(body, &form)
				_ = h.db.UpdateOpenWebUIFolderExpanded(subdomain, u.ID, folderID, form["is_expanded"])
				writeJSON(w, http.StatusOK, true)
				return
			}
		}

		if r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIFolder(subdomain, u.ID, folderID)
			writeJSON(w, http.StatusOK, true)
			return
		}
	}

	writeJSON(w, http.StatusOK, []any{})
}

// handleOpenWebUISkills handles /api/v1/skills/*
func (h *APIHandler) handleOpenWebUISkills(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/skills")
	path = strings.TrimPrefix(path, "/")

	if path == "list" {
		writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
			Items: []any{},
			Total: 0,
		})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

func knowledgeToResponse(k *db.OpenWebUIKnowledgeDB, u *OpenWebUISessionUserInfoResponse) map[string]any {
	if k == nil {
		return map[string]any{}
	}
	var meta any = map[string]any{}
	if k.MetaJSON != "" && k.MetaJSON != "{}" {
		_ = json.Unmarshal([]byte(k.MetaJSON), &meta)
	}
	var grants any = []any{}
	if k.AccessGrantsJSON != "" && k.AccessGrantsJSON != "[]" {
		_ = json.Unmarshal([]byte(k.AccessGrantsJSON), &grants)
	}
	return map[string]any{
		"id":            k.ID,
		"user_id":       k.UserID,
		"name":          k.Name,
		"description":   k.Description,
		"meta":          meta,
		"access_grants": grants,
		"files":         []any{},
		"created_at":    k.CreatedAt,
		"updated_at":    k.UpdatedAt,
		"write_access":  true,
		"file_count":    0,
		"user": map[string]any{
			"id":    u.ID,
			"name":  u.Name,
			"email": u.Email,
		},
	}
}

// handleOpenWebUIKnowledge handles /api/v1/knowledge/*
func (h *APIHandler) handleOpenWebUIKnowledge(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge")
	path = strings.TrimPrefix(path, "/")

	if path == "" || path == "/" || path == "search" {
		kbs, _ := h.db.GetOpenWebUIKnowledgeBases(subdomain, u.ID)
		query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
		var items []any
		for _, k := range kbs {
			if query != "" {
				lName := strings.ToLower(k.Name)
				lDesc := strings.ToLower(k.Description)
				if !strings.Contains(lName, query) && !strings.Contains(lDesc, query) {
					continue
				}
			}
			items = append(items, knowledgeToResponse(&k, u))
		}
		if items == nil {
			items = []any{}
		}
		total := len(items)
		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		limit := 30
		skip := (page - 1) * limit
		if skip >= total {
			items = []any{}
		} else {
			end := skip + limit
			if end > total {
				end = total
			}
			items = items[skip:end]
		}
		writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
			Items: items,
			Total: int64(total),
		})
		return
	}

	if path == "create" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		var form struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			AccessGrants []any  `json:"access_grants"`
		}
		_ = json.Unmarshal(body, &form)
		kID := uuid.New().String()
		now := time.Now().Unix()
		grantsJSON, _ := json.Marshal(form.AccessGrants)
		kb := db.OpenWebUIKnowledgeDB{
			ID:               kID,
			Subdomain:        subdomain,
			UserID:           u.ID,
			Name:             form.Name,
			Description:      form.Description,
			MetaJSON:         "{}",
			AccessGrantsJSON: string(grantsJSON),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		_ = h.db.UpsertOpenWebUIKnowledge(kb)
		writeJSON(w, http.StatusOK, knowledgeToResponse(&kb, u))
		return
	}

	if path == "reindex" || path == "metadata/reindex" {
		writeJSON(w, http.StatusOK, true)
		return
	}

	if strings.HasPrefix(path, "external/") {
		sub := strings.TrimPrefix(path, "external/")
		if sub == "connections" && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": true})
		return
	}

	// Sub-actions on {id}/*
	parts := strings.Split(path, "/")
	kID := parts[0]

	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			kb, err := h.db.GetOpenWebUIKnowledgeByID(subdomain, kID)
			if err != nil || kb == nil {
				writeJSONError(w, http.StatusNotFound, "Knowledge base not found")
				return
			}
			writeJSON(w, http.StatusOK, knowledgeToResponse(kb, u))
			return
		}
		if r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIKnowledge(subdomain, u.ID, kID)
			writeJSON(w, http.StatusOK, true)
			return
		}
	}

	if len(parts) >= 2 {
		action := parts[1]
		if action == "delete" {
			_ = h.db.DeleteOpenWebUIKnowledge(subdomain, u.ID, kID)
			writeJSON(w, http.StatusOK, true)
			return
		}
		if action == "update" {
			body, _ := io.ReadAll(r.Body)
			var form struct {
				Name         *string `json:"name"`
				Description  *string `json:"description"`
				AccessGrants []any   `json:"access_grants"`
			}
			_ = json.Unmarshal(body, &form)
			kb, err := h.db.GetOpenWebUIKnowledgeByID(subdomain, kID)
			if err == nil && kb != nil {
				if form.Name != nil {
					kb.Name = *form.Name
				}
				if form.Description != nil {
					kb.Description = *form.Description
				}
				if form.AccessGrants != nil {
					gJSON, _ := json.Marshal(form.AccessGrants)
					kb.AccessGrantsJSON = string(gJSON)
				}
				kb.UpdatedAt = time.Now().Unix()
				_ = h.db.UpsertOpenWebUIKnowledge(*kb)
				writeJSON(w, http.StatusOK, knowledgeToResponse(kb, u))
				return
			}
			writeJSONError(w, http.StatusNotFound, "Knowledge base not found")
			return
		}
		if action == "access" {
			body, _ := io.ReadAll(r.Body)
			var form struct {
				AccessGrants []any `json:"access_grants"`
			}
			_ = json.Unmarshal(body, &form)
			kb, err := h.db.GetOpenWebUIKnowledgeByID(subdomain, kID)
			if err == nil && kb != nil {
				gJSON, _ := json.Marshal(form.AccessGrants)
				kb.AccessGrantsJSON = string(gJSON)
				kb.UpdatedAt = time.Now().Unix()
				_ = h.db.UpsertOpenWebUIKnowledge(*kb)
				writeJSON(w, http.StatusOK, knowledgeToResponse(kb, u))
				return
			}
			writeJSONError(w, http.StatusNotFound, "Knowledge base not found")
			return
		}
		if action == "files" {
			if len(parts) >= 3 && parts[2] == "pending" {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"items":       []any{},
				"directories": []any{},
				"breadcrumbs": []any{},
				"total":       0,
			})
			return
		}
		if action == "file" || action == "reset" {
			kb, _ := h.db.GetOpenWebUIKnowledgeByID(subdomain, kID)
			writeJSON(w, http.StatusOK, knowledgeToResponse(kb, u))
			return
		}
		if action == "sync" {
			writeJSON(w, http.StatusOK, map[string]any{"status": true})
			return
		}
	}

	writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
		Items: []any{},
		Total: 0,
	})
}

// handleOpenWebUINotes handles /api/v1/notes/*
func (h *APIHandler) handleOpenWebUINotes(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/notes")
	path = strings.TrimPrefix(path, "/")

	if path == "search" {
		writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
			Items: []any{},
			Total: 0,
		})
		return
	}
	if path == "pinned" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// handleOpenWebUITools handles /api/v1/tools/*
func (h *APIHandler) handleOpenWebUITools(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tools")
	path = strings.TrimPrefix(path, "/")

	// Sub-actions on id/:id/*
	if strings.HasPrefix(path, "id/") {
		sub := strings.TrimPrefix(path, "id/")
		parts := strings.Split(sub, "/")
		toolID := parts[0]
		cleanID := strings.TrimPrefix(toolID, "server:mcp:")

		if len(parts) >= 2 && parts[1] == "call" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var form struct {
				Name   string `json:"name"`
				Params any    `json:"params"`
			}
			_ = json.Unmarshal(body, &form)

			servers, _ := h.db.GetOpenWebUIToolServers(subdomain)
			var targetServer *db.OpenWebUIToolServerDB
			for _, s := range servers {
				if s.ID == cleanID || s.ID == toolID {
					targetServer = &s
					break
				}
			}
			if targetServer != nil && targetServer.URL != "" {
				ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
				defer cancel()
				client := NewMCPClient()
				res, err := client.ExecuteTool(ctx, targetServer.URL, targetServer.APIKey, form.Name, form.Params)
				if err != nil {
					writeJSONError(w, http.StatusInternalServerError, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, res)
				return
			}
			writeJSONError(w, http.StatusNotFound, "Tool server not found")
			return
		}

		if len(parts) >= 2 && parts[1] == "valves" {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}

		if len(parts) == 1 && r.Method == http.MethodGet {
			servers, _ := h.db.GetOpenWebUIToolServers(subdomain)
			for _, s := range servers {
				if s.ID == cleanID || s.ID == toolID {
					var info map[string]any
					_ = json.Unmarshal([]byte(s.InfoJSON), &info)
					desc := ""
					var specs []any
					if info != nil {
						if d, ok := info["description"].(string); ok {
							desc = d
						}
						if sp, ok := info["specs"].([]any); ok {
							specs = sp
						}
					}
					if specs == nil {
						specs = []any{}
					}
					writeJSON(w, http.StatusOK, map[string]any{
						"id":           "server:mcp:" + s.ID,
						"user_id":      u.ID,
						"name":         s.Name,
						"meta":         map[string]any{"description": desc},
						"specs":        specs,
						"write_access": true,
						"updated_at":   s.UpdatedAt,
						"created_at":   s.CreatedAt,
					})
					return
				}
			}
			writeJSONError(w, http.StatusNotFound, "Tool not found")
			return
		}

		if len(parts) == 1 && r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIToolServer(subdomain, cleanID)
			_ = h.db.DeleteOpenWebUIToolServer(subdomain, toolID)
			writeJSON(w, http.StatusOK, map[string]any{"status": true})
			return
		}
	}

	// Also handle DELETE /api/v1/tools/:id
	if r.Method == http.MethodDelete && path != "" && !strings.Contains(path, "/") {
		cleanID := strings.TrimPrefix(path, "server:mcp:")
		_ = h.db.DeleteOpenWebUIToolServer(subdomain, cleanID)
		_ = h.db.DeleteOpenWebUIToolServer(subdomain, path)
		writeJSON(w, http.StatusOK, map[string]any{"status": true})
		return
	}

	servers, _ := h.db.GetOpenWebUIToolServers(subdomain)
	var tools []map[string]any
	for _, s := range servers {
		var cfg map[string]any
		_ = json.Unmarshal([]byte(s.ConfigJSON), &cfg)
		if enabled, ok := cfg["enable"].(bool); ok && !enabled {
			continue
		}
		var info map[string]any
		_ = json.Unmarshal([]byte(s.InfoJSON), &info)
		desc := ""
		var specs []any
		if info != nil {
			if d, ok := info["description"].(string); ok {
				desc = d
			}
			if sp, ok := info["specs"].([]any); ok {
				specs = sp
			}
		}
		if specs == nil {
			specs = []any{}
		}
		tools = append(tools, map[string]any{
			"id":           "server:mcp:" + s.ID,
			"user_id":      u.ID,
			"name":         s.Name,
			"meta":         map[string]any{"description": desc},
			"specs":        specs,
			"write_access": true,
			"updated_at":   s.UpdatedAt,
			"created_at":   s.CreatedAt,
		})
	}
	if tools == nil {
		tools = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, tools)
}

// handleOpenWebUIFunctions handles /api/v1/functions/*
func (h *APIHandler) handleOpenWebUIFunctions(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// handleOpenWebUIConfigs handles /api/v1/configs/*
func (h *APIHandler) handleOpenWebUIConfigs(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/configs")
	path = strings.TrimPrefix(path, "/")

	if path == "banners" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	if path == "tool_servers/verify" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		var form struct {
			URL      string            `json:"url"`
			Type     string            `json:"type"`
			AuthType string            `json:"auth_type"`
			Key      string            `json:"key"`
			Headers  map[string]string `json:"headers"`
		}
		_ = json.Unmarshal(body, &form)
		targetURL := strings.TrimSpace(form.URL)
		if targetURL == "" {
			writeJSONError(w, http.StatusBadRequest, "URL is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		client := NewMCPClient()
		tools, err := client.ListTools(ctx, targetURL, form.Key)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to connect to MCP server: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": true,
			"specs":  tools,
		})
		return
	}

	if path == "tool_servers" {
		if r.Method == http.MethodGet {
			servers, _ := h.db.GetOpenWebUIToolServers(subdomain)
			var connList []map[string]any
			for _, s := range servers {
				var cfg map[string]any
				_ = json.Unmarshal([]byte(s.ConfigJSON), &cfg)
				if cfg == nil {
					cfg = map[string]any{"enable": true}
				}
				var info map[string]any
				_ = json.Unmarshal([]byte(s.InfoJSON), &info)
				if info == nil {
					info = map[string]any{"id": s.ID, "name": s.Name}
				}
				connList = append(connList, map[string]any{
					"id":        s.ID,
					"name":      s.Name,
					"type":      s.Type,
					"url":       s.URL,
					"auth_type": s.AuthType,
					"key":       s.APIKey,
					"config":    cfg,
					"info":      info,
				})
			}
			if connList == nil {
				connList = []map[string]any{}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"TOOL_SERVER_CONNECTIONS": connList,
			})
			return
		}

		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var form struct {
				Connections []struct {
					ID       string         `json:"id"`
					Name     string         `json:"name"`
					Type     string         `json:"type"`
					URL      string         `json:"url"`
					AuthType string         `json:"auth_type"`
					Key      string         `json:"key"`
					Config   map[string]any `json:"config"`
					Info     map[string]any `json:"info"`
				} `json:"TOOL_SERVER_CONNECTIONS"`
			}
			_ = json.Unmarshal(body, &form)
			now := time.Now().Unix()
			keptIDs := make(map[string]bool)
			for _, c := range form.Connections {
				sID := c.ID
				if sID == "" {
					if c.Info != nil {
						if idVal, ok := c.Info["id"].(string); ok && idVal != "" {
							sID = idVal
						}
					}
				}
				if sID == "" {
					sID = uuid.New().String()
				}
				keptIDs[sID] = true
				sName := c.Name
				if sName == "" && c.Info != nil {
					if nameVal, ok := c.Info["name"].(string); ok && nameVal != "" {
						sName = nameVal
					}
				}
				if sName == "" {
					sName = "MCP Server"
				}
				cfgJSON, _ := json.Marshal(c.Config)
				infoJSON, _ := json.Marshal(c.Info)
				srvType := c.Type
				if srvType == "" {
					srvType = "mcp"
				}
				_ = h.db.UpsertOpenWebUIToolServer(db.OpenWebUIToolServerDB{
					ID:         sID,
					Subdomain:  subdomain,
					UserID:     u.ID,
					Name:       sName,
					Type:       srvType,
					URL:        c.URL,
					AuthType:   c.AuthType,
					APIKey:     c.Key,
					ConfigJSON: string(cfgJSON),
					InfoJSON:   string(infoJSON),
					CreatedAt:  now,
					UpdatedAt:  now,
				})
			}

			// Prune any existing server in DB that is not in the kept list
			existingServers, _ := h.db.GetOpenWebUIToolServers(subdomain)
			for _, es := range existingServers {
				if !keptIDs[es.ID] {
					_ = h.db.DeleteOpenWebUIToolServer(subdomain, es.ID)
				}
			}
			servers, _ := h.db.GetOpenWebUIToolServers(subdomain)
			var connList []map[string]any
			for _, s := range servers {
				var cfg map[string]any
				_ = json.Unmarshal([]byte(s.ConfigJSON), &cfg)
				var info map[string]any
				_ = json.Unmarshal([]byte(s.InfoJSON), &info)
				connList = append(connList, map[string]any{
					"id":        s.ID,
					"name":      s.Name,
					"type":      s.Type,
					"url":       s.URL,
					"auth_type": s.AuthType,
					"key":       s.APIKey,
					"config":    cfg,
					"info":      info,
				})
			}
			if connList == nil {
				connList = []map[string]any{}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"TOOL_SERVER_CONNECTIONS": connList,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// handleOpenWebUIAPIFallback handles any unmatched /api/ or /api/v1/ endpoints
func (h *APIHandler) handleOpenWebUIAPIFallback(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	path := strings.ToLower(r.URL.Path)

	// Endpoints expecting paginated response: {"items": [], "total": 0}
	if strings.HasSuffix(path, "/list") || strings.HasSuffix(path, "/search") ||
		strings.Contains(path, "knowledge") || strings.Contains(path, "files") ||
		strings.Contains(path, "skills/list") || strings.Contains(path, "prompts/list") {
		writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
			Items: []any{},
			Total: 0,
		})
		return
	}

	// Endpoints expecting an object/dict: configs, settings, info, status, details
	if strings.HasSuffix(path, "/config") || strings.HasSuffix(path, "/settings") ||
		strings.HasSuffix(path, "/details") || strings.HasSuffix(path, "/info") ||
		strings.HasSuffix(path, "/status") || strings.HasSuffix(path, "/valves") ||
		strings.HasSuffix(path, "/spec") {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}

	// Default for GET collection queries
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	// Default for POST/PUT/DELETE mutations
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
