package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"zcdns-backend/db"

	"github.com/google/uuid"
)

func (h *APIHandler) checkAdminAuth(r *http.Request) bool {
	// 1. Check header X-Admin-Key
	if key := r.Header.Get("X-Admin-Key"); key != "" && key == h.cfg.AdminKey {
		return true
	}

	// 2. Check Authorization Bearer header
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] == h.cfg.AdminKey {
			return true
		}
	}

	// 3. Check cookie zcdns_admin_token
	if cookie, err := r.Cookie("zcdns_admin_token"); err == nil && cookie.Value == h.cfg.AdminKey {
		return true
	}

	return false
}

func (h *APIHandler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.setCorsHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if !h.checkAdminAuth(r) {
			writeJSONError(w, http.StatusUnauthorized, "Akses administrator ditolak. Silakan login terlebih dahulu.")
			return
		}

		next(w, r)
	}
}

func (h *APIHandler) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		Key      string `json:"key"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Format payload tidak valid")
		return
	}

	provided := req.Key
	if provided == "" {
		provided = req.Password
	}

	if provided != h.cfg.AdminKey {
		writeJSONError(w, http.StatusUnauthorized, "Password admin tidak sesuai")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "zcdns_admin_token",
		Value:    h.cfg.AdminKey,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"token":   h.cfg.AdminKey,
		"message": "Login administrator berhasil",
	})
}

func (h *APIHandler) handleAdminGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetGrowthStats()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	reports, _ := h.db.GetAbuseReports("")
	pendingCount := 0
	investigatingCount := 0
	resolvedCount := 0
	for _, rep := range reports {
		switch rep.Status {
		case "pending":
			pendingCount++
		case "investigating":
			investigatingCount++
		case "resolved_blocked":
			resolvedCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"growth":           stats,
		"total_reports":    len(reports),
		"pending_reports":  pendingCount,
		"investigating":    investigatingCount,
		"resolved_blocked": resolvedCount,
	})
}

func (h *APIHandler) handleAdminGetAbuseReports(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	reports, err := h.db.GetAbuseReports(statusFilter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reports == nil {
		reports = make([]*db.AbuseReport, 0)
	}
	writeJSON(w, http.StatusOK, reports)
}

func (h *APIHandler) handleAdminUpdateAbuseReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Report ID is required")
		return
	}

	report, err := h.db.GetAbuseReportByID(id)
	if err != nil || report == nil {
		writeJSONError(w, http.StatusNotFound, "Laporan tidak ditemukan")
		return
	}

	var req struct {
		Status         string `json:"status"` // pending, investigating, resolved_blocked, dismissed
		AdminNotes     string `json:"admin_notes"`
		BlockSubdomain bool   `json:"block_subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if req.Status == "" {
		req.Status = report.Status
	}

	// If admin chooses to block the subdomain
	if req.BlockSubdomain || req.Status == "resolved_blocked" {
		reason := "Penyalahgunaan: " + req.Status
		if req.AdminNotes != "" {
			reason = req.AdminNotes
		}
		_ = h.db.BlockSubdomain(report.Subdomain, reason)
	}

	if err := h.db.UpdateAbuseReport(id, req.Status, req.AdminNotes); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := h.db.GetAbuseReportByID(id)
	writeJSON(w, http.StatusOK, updated)
}

func (h *APIHandler) handleAdminDeleteAbuseReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Report ID is required")
		return
	}

	if err := h.db.DeleteAbuseReport(id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *APIHandler) handleAdminGetBlockedSubdomains(w http.ResponseWriter, r *http.Request) {
	list, err := h.db.GetBlockedSubdomains()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = make([]map[string]any, 0)
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *APIHandler) handleAdminBlockSubdomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subdomain string `json:"subdomain"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	sub := strings.TrimSpace(strings.ToLower(req.Subdomain))
	sub = strings.TrimSuffix(sub, "."+strings.ToLower(h.cfg.BaseDomain))
	if sub == "" {
		writeJSONError(w, http.StatusBadRequest, "Subdomain is required")
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "Blocked by administrator"
	}

	if err := h.db.BlockSubdomain(sub, reason); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "subdomain": sub})
}

func (h *APIHandler) handleAdminUnblockSubdomain(w http.ResponseWriter, r *http.Request) {
	sub := r.PathValue("subdomain")
	sub = strings.TrimSpace(strings.ToLower(sub))
	sub = strings.TrimSuffix(sub, "."+strings.ToLower(h.cfg.BaseDomain))
	if sub == "" {
		writeJSONError(w, http.StatusBadRequest, "Subdomain is required")
		return
	}

	if err := h.db.UnblockSubdomain(sub); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "subdomain": sub})
}

type AdminTXTRequest struct {
	Subdomain string `json:"subdomain"`
	Name      string `json:"name"`
	Value     string `json:"value"`
	TTL       int    `json:"ttl"`
}

func (h *APIHandler) handleAdminCreateTXTRecord(w http.ResponseWriter, r *http.Request) {
	var req AdminTXTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
	if sub == "" {
		sub = "guard"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "_acme-challenge"
	}
	val := strings.Trim(strings.TrimSpace(req.Value), "\"")
	if val == "" {
		writeJSONError(w, http.StatusBadRequest, "TXT value is required")
		return
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = 60
	}

	record := &db.Record{
		ID:        uuid.New().String(),
		Subdomain: sub,
		Name:      name,
		Type:      "TXT",
		Value:     val,
		TTL:       ttl,
	}

	if err := h.db.AddRecord(record); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save TXT record: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, record)
	h.triggerRecordChanged()
}

func (h *APIHandler) handleAdminDeleteTXTRecord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Record ID is required")
		return
	}

	if err := h.db.DeleteRecordByID(id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete TXT record: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
	h.triggerRecordChanged()
}

func (h *APIHandler) handleAdminGetTXTRecords(w http.ResponseWriter, r *http.Request) {
	records, err := h.db.GetAllTXTRecords()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}
