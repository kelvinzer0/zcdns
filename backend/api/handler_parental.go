package api

import (
	"encoding/json"
	"net/http"
	"zcdns-backend/db"
)

func (h *APIHandler) handleGetParental(w http.ResponseWriter, r *http.Request, subdomain string) {
	cfg, err := h.db.GetParentalConfig(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load parental config: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *APIHandler) handleSaveParental(w http.ResponseWriter, r *http.Request, subdomain string) {
	var cfg db.ParentalConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	cfg.Subdomain = subdomain
	if err := h.db.SaveParentalConfig(&cfg); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save parental config: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}
