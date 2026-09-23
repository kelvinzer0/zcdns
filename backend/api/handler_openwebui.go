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

	"github.com/gorilla/websocket"
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
			"enable_websocket":          true,
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

	case path == "all/tags" || path == "tags" || strings.HasSuffix(path, "/tags"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case path == "all/archived" || path == "archived" || strings.HasPrefix(path, "folder/"):
		writeJSON(w, http.StatusOK, []any{})
		return

	case path == "archived/count":
		writeJSON(w, http.StatusOK, 0)
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

// handleOpenWebUISocketIO handles /ws/socket.io/ (WebSocket upgrade and polling fallback)
func (h *APIHandler) handleOpenWebUISocketIO(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Engine.IO v4 Handshake packet: 0{"sid":"zcdns","upgrades":[],"pingInterval":25000,"pingTimeout":20000}
		handshake := `0{"sid":"zcdns","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(handshake)); err != nil {
			return
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			s := string(msg)
			if s == "2" { // ping
				_ = conn.WriteMessage(websocket.TextMessage, []byte("3")) // pong
			} else if strings.HasPrefix(s, "40") { // Socket.IO CONNECT
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`40{"sid":"zcdns"}`))
			}
		}
		return
	}

	// Polling fallback
	if setOWUCors(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	if r.Method == http.MethodGet {
		w.Write([]byte(`0{"sid":"zcdns","upgrades":["websocket"],"pingInterval":25000,"pingTimeout":20000}`))
		return
	}
	if r.Method == http.MethodPost {
		w.Write([]byte("ok"))
		return
	}
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

const authBootstrapScript = `<script>
(function() {
  var p = new URLSearchParams(window.location.search);
  var k = p.get('key');
  if (k) {
    localStorage.setItem('token', k);
    localStorage.setItem('zcdns_key', k);
    p.delete('key');
    var s = p.toString() ? ('?' + p.toString()) : '';
    window.history.replaceState({}, document.title, window.location.pathname + s);
  }
  function checkKey() {
    var cur = localStorage.getItem('token') || localStorage.getItem('zcdns_key');
    if (!cur || cur === 'undefined' || cur === 'null' || cur.trim() === '') {
      showModal();
    }
  }
  function showModal() {
    if (document.getElementById('zcdns-key-modal')) return;
    var o = document.createElement('div');
    o.id = 'zcdns-key-modal';
    o.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.85);z-index:9999999;display:flex;align-items:center;justify-content:center;padding:16px;font-family:system-ui,-apple-system,sans-serif;';
    var b = document.createElement('div');
    b.style.cssText = 'background:#18181b;color:#f4f4f5;border:1px solid #27272a;max-width:400px;width:100%;padding:24px;box-shadow:0 25px 50px -12px rgba(0,0,0,0.5);';
    b.innerHTML = '<div style="margin-bottom:16px;"><h2 style="margin:0 0 4px 0;font-size:16px;font-weight:700;color:#fff;">AI Router Authentication</h2><p style="margin:0;font-size:12px;color:#a1a1aa;">Enter your client API key to connect to this router.</p></div><div style="margin-bottom:16px;"><label style="display:block;font-size:11px;font-weight:600;color:#d4d4d8;margin-bottom:6px;text-transform:uppercase;">Client API Key</label><input id="zcdns-key-input" type="password" placeholder="zck_..." style="width:100%;box-sizing:border-box;background:#09090b;border:1px solid #27272a;color:#fff;padding:8px 12px;font-size:13px;font-family:monospace;outline:none;" /></div><div style="display:flex;gap:8px;"><button id="zcdns-key-submit" style="flex:1;background:#fff;color:#000;border:none;padding:8px 14px;font-size:12px;font-weight:600;cursor:pointer;">Connect</button></div><div style="margin-top:14px;font-size:11px;color:#71717a;text-align:center;">Manage keys on <a href="https://zcdns.id" target="_blank" style="color:#60a5fa;text-decoration:underline;">ZCDNS Dashboard</a></div>';
    o.appendChild(b);
    document.body.appendChild(o);
    var inp = document.getElementById('zcdns-key-input');
    var btn = document.getElementById('zcdns-key-submit');
    if (inp) inp.focus();
    function submit() {
      var v = (inp && inp.value || '').trim();
      if (!v) { if (inp) inp.style.borderColor = '#ef4444'; return; }
      localStorage.setItem('token', v);
      localStorage.setItem('zcdns_key', v);
      o.remove();
      window.location.reload();
    }
    if (btn) btn.onclick = submit;
    if (inp) inp.onkeydown = function(e) { if (e.key === 'Enter') submit(); };
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', checkKey);
  } else {
    checkKey();
  }
})();
</script>`

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

	// SPA fallback: inject auth bootstrap script dynamically into index.html
	indexPath := filepath.Join(owuDir, "index.html")
	htmlBytes, err := os.ReadFile(indexPath)
	if err != nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	htmlStr := string(htmlBytes)
	if strings.Contains(htmlStr, "<head>") {
		htmlStr = strings.Replace(htmlStr, "<head>", "<head>"+authBootstrapScript, 1)
	} else {
		htmlStr = authBootstrapScript + htmlStr
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlStr))
}
