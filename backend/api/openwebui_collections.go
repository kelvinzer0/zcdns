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

// handleOpenWebUIPrompts handles /api/v1/prompts/* with DB backing
func (h *APIHandler) handleOpenWebUIPrompts(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts")
	path = strings.TrimPrefix(path, "/")

	if path == "list" {
		prompts, _ := h.db.GetOpenWebUIPrompts(subdomain)
		var items []any
		for _, p := range prompts {
			items = append(items, map[string]any{
				"command":    p.Command,
				"name":       p.Name,
				"content":    p.Content,
				"user_id":    "admin",
				"created_at": p.CreatedAt,
				"updated_at": p.UpdatedAt,
			})
		}
		if items == nil {
			items = []any{}
		}
		writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
			Items: items,
			Total: int64(len(items)),
		})
		return
	}

	if path == "create" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		var form map[string]string
		_ = json.Unmarshal(body, &form)
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		pID := hex.EncodeToString(randBytes)
		_ = h.db.UpsertOpenWebUIPrompt(subdomain, pID, form["command"], form["name"], form["content"])
		writeJSON(w, http.StatusOK, map[string]any{"command": form["command"], "name": form["name"]})
		return
	}

	if path == "tags" {
		writeJSON(w, http.StatusOK, []string{})
		return
	}

	prompts, _ := h.db.GetOpenWebUIPrompts(subdomain)
	var list []any
	for _, p := range prompts {
		list = append(list, map[string]any{
			"command": p.Command,
			"name":    p.Name,
			"content": p.Content,
		})
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
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/folders")
	path = strings.TrimPrefix(path, "/")

	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			rawFolders, err := h.db.GetOpenWebUIFolders(subdomain)
			if err != nil {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
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
					UnreadCount: 0,
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
			_ = h.db.CreateOpenWebUIFolder(subdomain, folderID, name, parentID)

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

	// Sub-actions on /folders/{id}/*
	if path != "" {
		parts := strings.Split(path, "/")
		folderID := parts[0]
		subAction := ""
		if len(parts) > 1 {
			subAction = parts[1]
		}

		if subAction == "read" {
			writeJSON(w, http.StatusOK, map[string]int{"updated_count": 0})
			return
		}
		if subAction == "update" {
			body, _ := io.ReadAll(r.Body)
			var form map[string]string
			_ = json.Unmarshal(body, &form)
			if name, ok := form["name"]; ok && name != "" {
				_ = h.db.UpdateOpenWebUIFolderName(subdomain, folderID, name)
			}
			writeJSON(w, http.StatusOK, true)
			return
		}
		if subAction == "expanded" {
			body, _ := io.ReadAll(r.Body)
			var form map[string]bool
			_ = json.Unmarshal(body, &form)
			_ = h.db.UpdateOpenWebUIFolderExpanded(subdomain, folderID, form["is_expanded"])
			writeJSON(w, http.StatusOK, true)
			return
		}
		if r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIFolder(subdomain, folderID)
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

// handleOpenWebUIKnowledge handles /api/v1/knowledge/*
func (h *APIHandler) handleOpenWebUIKnowledge(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, OpenWebUIPaginatedListResponse{
		Items: []any{},
		Total: 0,
	})
}

// handleOpenWebUIFiles handles /api/v1/files/*
func (h *APIHandler) handleOpenWebUIFiles(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files")
	path = strings.TrimPrefix(path, "/")

	if path == "count" {
		writeJSON(w, http.StatusOK, 0)
		return
	}
	if path == "search" {
		writeJSON(w, http.StatusOK, []any{})
		return
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
	writeJSON(w, http.StatusOK, []any{})
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
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/configs")
	path = strings.TrimPrefix(path, "/")

	if path == "banners" {
		writeJSON(w, http.StatusOK, []any{})
		return
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
