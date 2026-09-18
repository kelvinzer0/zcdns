package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"zcdns-backend/db"

	"github.com/google/uuid"
)

type AbuseSubmitRequest struct {
	ReporterName  string `json:"reporter_name"`
	ReporterEmail string `json:"reporter_email"`
	AbuseType     string `json:"abuse_type"`
	Subdomain     string `json:"subdomain"`
	Description   string `json:"description"`
	Evidence      string `json:"evidence"`
}

func (h *APIHandler) handleSubmitAbuse(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req AbuseSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Format payload tidak valid")
		return
	}

	req.ReporterName = strings.TrimSpace(req.ReporterName)
	req.ReporterEmail = strings.TrimSpace(req.ReporterEmail)
	req.Subdomain = strings.TrimSpace(strings.ToLower(req.Subdomain))
	req.Description = strings.TrimSpace(req.Description)

	if req.ReporterEmail == "" || !strings.Contains(req.ReporterEmail, "@") {
		writeJSONError(w, http.StatusBadRequest, "Alamat email pelapor tidak valid")
		return
	}

	if req.Subdomain == "" {
		writeJSONError(w, http.StatusBadRequest, "Subdomain yang dilaporkan wajib diisi")
		return
	}

	// Clean subdomain if user typed http://, https://, or .zcdns.id
	cleanSub := strings.TrimPrefix(req.Subdomain, "http://")
	cleanSub = strings.TrimPrefix(cleanSub, "https://")
	cleanSub = strings.TrimSuffix(cleanSub, "."+strings.ToLower(h.cfg.BaseDomain))
	cleanSub = strings.TrimSuffix(cleanSub, "/")
	if parts := strings.Split(cleanSub, "."); len(parts) > 1 {
		cleanSub = parts[0]
	}

	if req.Description == "" {
		writeJSONError(w, http.StatusBadRequest, "Deskripsi laporan penyalahgunaan wajib diisi")
		return
	}

	if req.AbuseType == "" {
		req.AbuseType = "other"
	}

	report := &db.AbuseReport{
		ID:            uuid.New().String(),
		ReporterName:  req.ReporterName,
		ReporterEmail: req.ReporterEmail,
		AbuseType:     req.AbuseType,
		Subdomain:     cleanSub,
		Description:   req.Description,
		Evidence:      req.Evidence,
		Status:        "pending",
		AdminNotes:    "",
	}

	if err := h.db.CreateAbuseReport(report); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal menyimpan laporan: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Laporan penyalahgunaan berhasil dikirim. Tim administrator ZCDNS akan meninjau laporan ini secepatnya.",
		"id":      report.ID,
	})
}
