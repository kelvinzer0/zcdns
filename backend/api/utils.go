package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (h *APIHandler) getSubdomain(r *http.Request) string {
	cleanSub := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.TrimSuffix(s, ".guard")
		return s
	}

	// 1. Path value (if route has {subdomain})
	if sub := r.PathValue("subdomain"); sub != "" {
		return cleanSub(sub)
	}
	// 2. Query param
	if sub := r.URL.Query().Get("subdomain"); sub != "" {
		return cleanSub(sub)
	}
	// 3. Header
	if sub := r.Header.Get("X-Subdomain"); sub != "" {
		return cleanSub(sub)
	}
	// 4. Host header (e.g. fox42.guard.zcdns.id)
	if host := r.Host; host != "" {
		cleanHost := strings.ToLower(host)
		if idx := strings.Index(cleanHost, ":"); idx != -1 {
			cleanHost = cleanHost[:idx]
		}
		base := strings.ToLower(h.cfg.BaseDomain)
		guardSuffix := ".guard." + base
		if strings.HasSuffix(cleanHost, guardSuffix) {
			sub := strings.TrimSuffix(cleanHost, guardSuffix)
			if sub != "" && !strings.Contains(sub, ".") {
				return sub
			}
		}
	}
	// 5. Cookie
	if c, err := r.Cookie("zcdns_subdomain"); err == nil && c.Value != "" {
		return cleanSub(c.Value)
	}
	return ""
}

func (h *APIHandler) requireSubdomain(next func(w http.ResponseWriter, r *http.Request, subdomain string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.setCorsHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		subdomain := h.getSubdomain(r)
		if subdomain == "" {
			writeJSONError(w, http.StatusUnauthorized, "Subdomain required. Please create or resume a session.")
			return
		}

		next(w, r, subdomain)
	}
}

func (h *APIHandler) setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Subdomain, Authorization, X-Admin-Key")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
