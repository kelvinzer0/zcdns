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

// handleOpenWebUIPrompts handles /api/v1/prompts/*
func (h *APIHandler) handleOpenWebUIPrompts(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts")
	path = strings.TrimPrefix(path, "/")

	if path == "list" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": []any{},
			"total": 0,
		})
		return
	}
	if path == "tags" {
		writeJSON(w, http.StatusOK, []string{})
		return
	}
	// Default GET /
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
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": []any{},
			"total": 0,
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
	// KnowledgeAccessListResponse requires {"items": [], "total": 0}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": []any{},
		"total": 0,
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
	// FileListResponse requires {"items": [], "total": 0}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": []any{},
		"total": 0,
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
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": []any{},
			"total": 0,
		})
		return
	}
	if path == "pinned" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// handleOpenWebUIFolders handles /api/v1/folders/*
func (h *APIHandler) handleOpenWebUIFolders(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/folders")
	path = strings.TrimPrefix(path, "/")

	if r.Method == http.MethodPost {
		if strings.HasSuffix(path, "/read") {
			writeJSON(w, http.StatusOK, map[string]int{"updated_count": 0})
			return
		}
		// Create new folder
		body, _ := io.ReadAll(r.Body)
		var form map[string]interface{}
		_ = json.Unmarshal(body, &form)

		name := "New Folder"
		if n, ok := form["name"].(string); ok && n != "" {
			name = n
		}
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		folderID := hex.EncodeToString(randBytes)
		now := time.Now().Unix()

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":           folderID,
			"name":         name,
			"parent_id":    nil,
			"is_expanded":  false,
			"unread_count": 0,
			"created_at":   now,
			"updated_at":   now,
		})
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
