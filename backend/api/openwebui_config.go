package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func sanitizeUserEmail(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var sb strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			sb.WriteRune(ch)
		} else if ch == ' ' {
			sb.WriteRune('-')
		}
	}
	s := sb.String()
	if s == "" {
		return "user"
	}
	return s
}

// resolveUserFromToken dynamically resolves the user identity from token / API key
func (h *APIHandler) resolveUserFromToken(subdomain, token string) *OpenWebUISessionUserInfoResponse {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	var userID string
	var userName string
	var email string

	if token != "" && token != "zcdns-session-token" {
		// 1. Check if token matches an active AIRouterUserKey
		if k, err := h.db.GetAIRouterUserKeyByValue(subdomain, token); err == nil && k != nil {
			userID = fmt.Sprintf("usr_%d", k.ID)
			userName = k.Name
			email = fmt.Sprintf("%s@%s.router.zcdns.id", sanitizeUserEmail(k.Name), subdomain)
		} else {
			// 2. Deterministic hashing based on the client token
			sum := sha256.Sum256([]byte(token))
			hash := hex.EncodeToString(sum[:])[:12]
			userID = "usr_" + hash
			userName = "User " + hash[:4]
			email = fmt.Sprintf("user-%s@%s.router.zcdns.id", hash[:6], subdomain)
		}
	} else {
		// 3. No token provided: generate a random guest token
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		randHex := hex.EncodeToString(randBytes)
		token = "zck_" + randHex
		userID = "usr_" + randHex[:8]
		userName = "User " + randHex[:4]
		email = fmt.Sprintf("user-%s@%s.router.zcdns.id", randHex[:6], subdomain)
	}

	return &OpenWebUISessionUserInfoResponse{
		Token:           token,
		TokenType:       "bearer",
		ID:              userID,
		Name:            userName,
		Role:            "user",
		Email:           email,
		ProfileImageURL: "/user.png",
		Permissions: map[string]any{
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
}

// resolveUser resolves user session from request headers, query params, or cookies
func (h *APIHandler) resolveUser(r *http.Request) *OpenWebUISessionUserInfoResponse {
	subdomain := h.resolveSubdomain(r)
	token := ""

	// 1. Authorization header: Bearer <token>
	if auth := r.Header.Get("Authorization"); auth != "" {
		token = strings.TrimPrefix(auth, "Bearer ")
		token = strings.TrimSpace(token)
	}

	// 2. Query parameters ?token= or ?key= or ?api_key=
	if token == "" {
		if q := r.URL.Query().Get("token"); q != "" {
			token = q
		} else if q := r.URL.Query().Get("key"); q != "" {
			token = q
		} else if q := r.URL.Query().Get("api_key"); q != "" {
			token = q
		}
	}

	// 3. Cookies
	if token == "" {
		if c, err := r.Cookie("token"); err == nil && c.Value != "" {
			token = c.Value
		} else if c, err := r.Cookie("zcdns_user_token"); err == nil && c.Value != "" {
			token = c.Value
		}
	}

	// 4. Custom headers
	if token == "" {
		if k := r.Header.Get("X-API-Key"); k != "" {
			token = k
		} else if k := r.Header.Get("X-Token"); k != "" {
			token = k
		}
	}

	return h.resolveUserFromToken(subdomain, token)
}

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
			"enable_websocket":          true,
		},
		"oauth": map[string]interface{}{
			"providers": map[string]interface{}{},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleOpenWebUIVersion handles /api/version and /api/version/updates
func (h *APIHandler) handleOpenWebUIVersion(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version": "0.5.0",
		"current": "0.5.0",
		"latest":  "0.5.0",
	})
}

// handleOpenWebUIAuth handles GET /api/v1/auths and /api/v1/users/user
func (h *APIHandler) handleOpenWebUIAuth(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	u := h.resolveUser(r)

	// Persist cookie for future requests and WebSocket handshake
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    u.Token,
		Path:     "/",
		MaxAge:   86400 * 365,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "zcdns_user_token",
		Value:    u.Token,
		Path:     "/",
		MaxAge:   86400 * 365,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, u)
}

// handleOpenWebUIUserSettings handles /api/v1/users/user/settings with DB persistence
func (h *APIHandler) handleOpenWebUIUserSettings(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)

	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		_ = h.db.SetOpenWebUIUserSettings(subdomain, u.ID, string(body))
		var settings map[string]interface{}
		_ = json.Unmarshal(body, &settings)
		writeJSON(w, http.StatusOK, settings)
		return
	}

	settingsJSON, err := h.db.GetOpenWebUIUserSettings(subdomain, u.ID)
	if err == nil && settingsJSON != "" && settingsJSON != "{}" {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(settingsJSON), &settings); err == nil {
			writeJSON(w, http.StatusOK, settings)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ui": map[string]interface{}{},
	})
}

// handleOpenWebUITasks handles /api/v1/tasks/config
func (h *APIHandler) handleOpenWebUITasks(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"TASK_MODEL":                 "",
		"TASK_MODEL_EXTERNAL":        "",
		"ENABLE_TITLE_GENERATION":    true,
		"ENABLE_TAGS_GENERATION":     true,
		"ENABLE_AUTOCOMPLETE_GENERATION": false,
	})
}

// handleOpenWebUIModels handles GET /api/models, GET /api/v1/models, and /api/v1/models/*
func (h *APIHandler) handleOpenWebUIModels(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	if strings.HasSuffix(r.URL.Path, "/list") {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": []any{},
			"total": 0,
		})
		return
	}
	if strings.HasSuffix(r.URL.Path, "/tags") || strings.HasSuffix(r.URL.Path, "/base/tags") {
		writeJSON(w, http.StatusOK, []string{})
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
		return fmt.Sprintf("%dk", sz/1000)
	}
	return "ctx"
}
