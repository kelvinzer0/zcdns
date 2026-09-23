package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// setOWUCors sets CORS headers for OpenWebUI requests
func setOWUCors(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")
	w.Header().Set("Access-Control-Max-Age", "86400")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// resolveSubdomain extracts subdomain from host or query/headers
func (h *APIHandler) resolveSubdomain(r *http.Request) string {
	sub := extractSubdomainFromHost(r.Host)
	if sub != "" {
		return sub
	}
	if q := r.URL.Query().Get("subdomain"); q != "" {
		return q
	}
	if h := r.Header.Get("X-Subdomain"); h != "" {
		return h
	}
	return "default"
}

// handleOpenWebUIConfig handles GET /api/config
func (h *APIHandler) handleOpenWebUIConfig(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	subdomain := h.resolveSubdomain(r)
	defaultModel := "gpt-4o-mini"
	if aliases, err := h.db.GetAIRouterAliases(subdomain); err == nil && len(aliases) > 0 {
		defaultModel = aliases[0].AliasName
	} else if combos, err := h.db.GetAIRouterCombos(subdomain); err == nil && len(combos) > 0 {
		defaultModel = combos[0].Name
	}

	resp := map[string]interface{}{
		"status":         true,
		"name":           "ZCDNS AI Router",
		"version":        "0.5.0",
		"default_locale": "en",
		"default_models": defaultModel,
		"features": map[string]interface{}{
			"auth":                      false,
			"auth_trusted_header":       false,
			"enable_signup":             false,
			"enable_login_form":         false,
			"enable_api_keys":           true,
			"enable_direct_connections": true,
			"enable_folders":            true,
			"enable_channels":           false,
			"enable_notes":              false,
			"enable_web_search":         false,
			"enable_image_generation":   false,
			"enable_community_sharing":  false,
		},
		"oauth": map[string]interface{}{
			"providers": map[string]interface{}{},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleOpenWebUIAuth handles GET /api/v1/auths and /api/v1/users/user
func (h *APIHandler) handleOpenWebUIAuth(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	subdomain := h.resolveSubdomain(r)
	user := map[string]interface{}{
		"id":                "admin",
		"email":             "user@" + subdomain + ".router.zcdns.id",
		"name":              subdomain,
		"role":              "admin",
		"profile_image_url": "/user.png",
		"token":             "zcdns-session-token",
		"token_type":        "bearer",
		"permissions": map[string]interface{}{
			"workspace": map[string]bool{
				"models":    true,
				"knowledge": true,
				"prompts":   true,
				"tools":     true,
			},
			"chat": map[string]bool{
				"controls":    true,
				"file_upload": true,
				"delete":      true,
				"edit":        true,
				"share":       true,
				"export":      true,
			},
		},
	}

	writeJSON(w, http.StatusOK, user)
}

// handleOpenWebUIModels handles GET /api/models and GET /api/v1/models
func (h *APIHandler) handleOpenWebUIModels(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	subdomain := h.resolveSubdomain(r)
	seen := map[string]bool{}
	var models []map[string]interface{}

	addModel := func(id, name, ownedBy string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		models = append(models, map[string]interface{}{
			"id":       id,
			"name":     name,
			"object":   "model",
			"owned_by": ownedBy,
		})
	}

	// 1. Aliases
	if aliases, err := h.db.GetAIRouterAliases(subdomain); err == nil {
		for _, a := range aliases {
			label := a.AliasName
			if a.ContextSize > 0 {
				label += " (" + formatCtx(a.ContextSize) + ")"
			}
			addModel(a.AliasName, label, "alias")
		}
	}

	// 2. Combos
	if combos, err := h.db.GetAIRouterCombos(subdomain); err == nil {
		for _, c := range combos {
			addModel(c.Name, c.Name+" ["+c.Strategy+"]", "combo")
		}
	}

	// 3. Provider connection models
	if conns, err := h.db.GetAIRouterConnections(subdomain); err == nil {
		for _, c := range conns {
			if c.Status != "active" {
				continue
			}
			switch c.Provider {
			case "anthropic":
				addModel("claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet", "anthropic")
				addModel("claude-3-5-haiku-20241022", "Claude 3.5 Haiku", "anthropic")
			case "groq":
				addModel("groq/llama-3.3-70b-versatile", "Llama 3.3 70B (Groq)", "groq")
				addModel("groq/llama-3.1-8b-instant", "Llama 3.1 8B (Groq)", "groq")
			case "together":
				addModel("meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", "Llama 3.1 70B Turbo", "together")
			case "openrouter":
				addModel("openrouter/auto", "OpenRouter Auto", "openrouter")
			default: // openai or custom
				addModel("gpt-4o", "GPT-4o", "openai")
				addModel("gpt-4o-mini", "GPT-4o Mini", "openai")
				addModel("o1", "o1", "openai")
				addModel("o3-mini", "o3-mini", "openai")
			}
		}
	}

	// Always ensure minimum defaults so UI is never empty
	addModel("gpt-4o", "GPT-4o", "zcdns")
	addModel("gpt-4o-mini", "GPT-4o Mini", "zcdns")
	addModel("claude-3-5-sonnet", "Claude 3.5 Sonnet", "zcdns")
	addModel("groq/llama-3.3-70b-versatile", "Llama 3.3 70B (Groq)", "zcdns")

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": models})
}

func formatCtx(sz int) string {
	if sz >= 1000 {
		return string(rune(sz/1000)) + "k"
	}
	return "ctx"
}

// handleOpenWebUIChats handles GET/POST/DELETE on /api/v1/chats
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
			chats, err := h.db.GetOpenWebUIChats(subdomain)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, chats)
			return
		}
		if r.Method == http.MethodDelete {
			_ = h.db.DeleteAllOpenWebUIChats(subdomain)
			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}

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
				"title":      title,
				"chat":       parsed["chat"],
				"created_at": now,
				"updated_at": now,
			})
			return
		}

	default: // /api/v1/chats/{id}
		chatID := path
		if r.Method == http.MethodGet {
			title, chatJSON, createdAt, updatedAt, err := h.db.GetOpenWebUIChatRaw(subdomain, chatID)
			if err != nil {
				writeJSONError(w, http.StatusNotFound, "chat not found")
				return
			}
			var chatData map[string]interface{}
			_ = json.Unmarshal([]byte(chatJSON), &chatData)
			if c, ok := chatData["chat"]; ok {
				chatData = map[string]interface{}{
					"id":         chatID,
					"title":      title,
					"chat":       c,
					"created_at": createdAt,
					"updated_at": updatedAt,
				}
			}
			writeJSON(w, http.StatusOK, chatData)
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
				"title":      title,
				"chat":       parsed["chat"],
				"updated_at": now,
			})
			return
		}
		if r.Method == http.MethodDelete {
			_ = h.db.DeleteOpenWebUIChat(subdomain, chatID)
			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

// locateOpenWebUIDir resolves the directory containing static OpenWebUI files
func (h *APIHandler) locateOpenWebUIDir() string {
	candidates := []string{
		filepath.Join(h.cfg.StaticDir, "openwebui"),
		"/opt/zcdns/dist/openwebui",
		"./dist/openwebui",
		"../dist/openwebui",
	}
	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
				return dir
			}
		}
	}
	return ""
}

// handleOpenWebUIStatic serves static assets or index.html for OpenWebUI SPA
func (h *APIHandler) handleOpenWebUIStatic(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	owuDir := h.locateOpenWebUIDir()
	if owuDir == "" {
		http.Error(w, "OpenWebUI frontend assets not installed yet", http.StatusServiceUnavailable)
		return
	}

	cleanPath := filepath.Clean(r.URL.Path)
	target := filepath.Join(owuDir, cleanPath)
	if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
		http.ServeFile(w, r, target)
		return
	}

	// SPA fallback
	http.ServeFile(w, r, filepath.Join(owuDir, "index.html"))
}
