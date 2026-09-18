package api

import "net/http"

func (h *APIHandler) handleDoH(w http.ResponseWriter, r *http.Request) {
	subdomain := h.getSubdomain(r)
	if subdomain == "" {
		subdomain = "default"
	}
	if h.parental != nil {
		h.parental.HandleDoH(w, r, subdomain)
	} else {
		http.Error(w, "DNS resolver engine not available", http.StatusServiceUnavailable)
	}
}
