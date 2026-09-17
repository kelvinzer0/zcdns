package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"zcdns-backend/config"
	"zcdns-backend/db"
	"zcdns-backend/parental"
)

var animals = []string{
	"orca", "otter", "falcon", "panda", "fox", "badger", "lynx", "tiger",
	"koala", "dolphin", "seal", "walrus", "eagle", "hawk", "owl", "wolf",
	"bear", "gecko", "bison", "cheetah", "leopard", "jaguar", "otter", "beaver",
}

type APIHandler struct {
	cfg             *config.Config
	db              *db.DB
	hub             *StreamHub
	parental        *parental.Engine
	onRecordChanged func()
}

func NewAPIHandler(cfg *config.Config, database *db.DB, hub *StreamHub, pe *parental.Engine) *APIHandler {
	return &APIHandler{
		cfg:      cfg,
		db:       database,
		hub:      hub,
		parental: pe,
	}
}

func (h *APIHandler) SetOnRecordChanged(fn func()) {
	h.onRecordChanged = fn
}

func (h *APIHandler) triggerRecordChanged() {
	if h.onRecordChanged != nil {
		go h.onRecordChanged()
	}
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// CORS and JSON middleware wrappers
	mux.HandleFunc("POST /api/session", h.handleCreateSession)
	mux.HandleFunc("GET /api/session", h.handleGetSession)
	mux.HandleFunc("DELETE /api/session", h.handleDeleteSession)
	mux.HandleFunc("DELETE /api/subdomain", h.requireSubdomain(h.handleDeleteSubdomain))
	mux.HandleFunc("POST /api/session/renew", h.requireSubdomain(h.handleRenewSession))

	mux.HandleFunc("GET /api/records", h.requireSubdomain(h.handleGetRecords))
	mux.HandleFunc("POST /api/records", h.requireSubdomain(h.handleCreateRecord))
	mux.HandleFunc("PUT /api/records/{id}", h.requireSubdomain(h.handleUpdateRecord))
	mux.HandleFunc("DELETE /api/records/{id}", h.requireSubdomain(h.handleDeleteRecord))
	mux.HandleFunc("DELETE /api/records", h.requireSubdomain(h.handleDeleteAllRecords))

	mux.HandleFunc("GET /api/requests", h.requireSubdomain(h.handleGetRequests))
	mux.HandleFunc("DELETE /api/requests", h.requireSubdomain(h.handleDeleteRequests))

	mux.HandleFunc("GET /api/parental", h.requireSubdomain(h.handleGetParental))
	mux.HandleFunc("POST /api/parental", h.requireSubdomain(h.handleSaveParental))

	mux.HandleFunc("GET /dns-query", h.handleDoH)
	mux.HandleFunc("POST /dns-query", h.handleDoH)
	mux.HandleFunc("GET /dns-query/{subdomain}", h.handleDoH)
	mux.HandleFunc("POST /dns-query/{subdomain}", h.handleDoH)
	mux.HandleFunc("GET /dns-query/guard/{subdomain}", h.handleDoH)
	mux.HandleFunc("POST /dns-query/guard/{subdomain}", h.handleDoH)

	mux.HandleFunc("GET /api/requeststream", h.handleWebSocketStream)
	// In addition, support /requeststream/{subdomain} or query param
	mux.HandleFunc("GET /requeststream", h.handleWebSocketStream)
	mux.HandleFunc("GET /requeststream/{subdomain}", h.handleWebSocketStream)

	mux.HandleFunc("POST /api/test-query", h.handleTestQuery)

	// Public Growth Stats & Abuse Report
	mux.HandleFunc("GET /api/stats", h.handleGetGrowthStats)
	mux.HandleFunc("POST /api/abuse", h.handleSubmitAbuse)

	// ACME DNS-01 Challenge Management
	mux.HandleFunc("POST /api/acme-challenge", h.handleCreateAcmeChallenge)
	mux.HandleFunc("GET /api/acme-challenge", h.handleGetAcmeChallenges)

	// Admin Management Routes
	mux.HandleFunc("POST /api/admin/login", h.handleAdminLogin)
	mux.HandleFunc("GET /api/admin/stats", h.requireAdmin(h.handleAdminGetStats))
	mux.HandleFunc("GET /api/admin/abuse-reports", h.requireAdmin(h.handleAdminGetAbuseReports))
	mux.HandleFunc("PATCH /api/admin/abuse-reports/{id}", h.requireAdmin(h.handleAdminUpdateAbuseReport))
	mux.HandleFunc("DELETE /api/admin/abuse-reports/{id}", h.requireAdmin(h.handleAdminDeleteAbuseReport))
	mux.HandleFunc("GET /api/admin/blocked-subdomains", h.requireAdmin(h.handleAdminGetBlockedSubdomains))
	mux.HandleFunc("POST /api/admin/blocked-subdomains", h.requireAdmin(h.handleAdminBlockSubdomain))
	mux.HandleFunc("DELETE /api/admin/blocked-subdomains/{subdomain}", h.requireAdmin(h.handleAdminUnblockSubdomain))
	mux.HandleFunc("GET /api/admin/txt-records", h.requireAdmin(h.handleAdminGetTXTRecords))
	mux.HandleFunc("POST /api/admin/txt-records", h.requireAdmin(h.handleAdminCreateTXTRecord))
	mux.HandleFunc("DELETE /api/admin/txt-records/{id}", h.requireAdmin(h.handleAdminDeleteTXTRecord))

	// Static SPA file server
	if h.cfg.StaticDir != "" {
		fileServer := http.FileServer(http.Dir(h.cfg.StaticDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(h.cfg.StaticDir, filepath.Clean(r.URL.Path))
			if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(h.cfg.StaticDir, "index.html"))
		})
	}
}

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

func (h *APIHandler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Capacity check (max 9,999 records for zone transfer stability)
	totalRecs, err := h.db.GetTotalRecordCount()
	if err == nil && totalRecs >= 9995 {
		writeJSONError(w, http.StatusServiceUnavailable, "ZeroCentDNS saat ini mencapai batas kapasitas maksimum (9.999 record). Mohon menunggu beberapa saat hingga kapasitas tersedia kembali.")
		return
	}

	// Delete old subdomain and its records if old subdomain is provided or present in active session
	oldSub := r.Header.Get("X-Delete-Old-Subdomain")
	if oldSub == "" {
		oldSub = h.getSubdomain(r)
	}
	if oldSub != "" {
		if err := h.db.DeleteSubdomain(oldSub); err != nil {
			log.Printf("[SESSION] Warning: Failed to delete old subdomain '%s': %v", oldSub, err)
		} else {
			log.Printf("[SESSION] Deleted old subdomain '%s' and all its records upon new subdomain request", oldSub)
		}
	}

	// Generate random unique subdomain: animal + random number 10-99
	var sub string
	for attempts := 0; attempts < 10; attempts++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(animals))))
		num, _ := rand.Int(rand.Reader, big.NewInt(90))
		candidate := fmt.Sprintf("%s%d", animals[idx.Int64()], num.Int64()+10)
		if _, err := h.db.GetUserBySubdomain(candidate); err != nil {
			sub = candidate
			break
		}
	}

	if sub == "" {
		sub = fmt.Sprintf("dns%d", time.Now().Unix()%10000)
	}

	userID := uuid.New().String()
	if err := h.db.CreateUser(userID, sub); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create user session: "+err.Error())
		return
	}

	// Create default educational initial records
	defaultA := &db.Record{
		ID:        uuid.New().String(),
		Subdomain: sub,
		Name:      "@",
		Type:      "A",
		Value:     "192.0.2.1", // RFC 5737 TEST-NET-1 (Safe Documentation / Educational IP)
		TTL:       60,
	}
	_ = h.db.AddRecord(defaultA)

	defaultTXT := &db.Record{
		ID:        uuid.New().String(),
		Subdomain: sub,
		Name:      "@",
		Type:      "TXT",
		Value:     "\"ZeroCentDNS Sandbox - Free DNS Education\"",
		TTL:       60,
	}
	_ = h.db.AddRecord(defaultTXT)

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "zcdns_subdomain",
		Value:    sub,
		Path:     "/",
		Expires:  time.Now().Add(14 * 24 * time.Hour),
		HttpOnly: false, // accessible to client js
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         userID,
		"subdomain":  sub,
		"domain":     fmt.Sprintf("%s.%s", sub, h.cfg.BaseDomain),
		"baseDomain": h.cfg.BaseDomain,
		"dnsPort":    h.cfg.DNSPort,
	})
	h.triggerRecordChanged()
}

func (h *APIHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	sub := h.getSubdomain(r)
	if sub == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"logged_in":  false,
			"baseDomain": h.cfg.BaseDomain,
			"dnsPort":    h.cfg.DNSPort,
		})
		return
	}

	user, err := h.db.GetUserBySubdomain(sub)
	if err != nil || user == nil {
		// Cookie is invalid or expired
		http.SetCookie(w, &http.Cookie{
			Name:    "zcdns_subdomain",
			Value:   "",
			Path:    "/",
			Expires: time.Unix(0, 0),
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"logged_in":  false,
			"baseDomain": h.cfg.BaseDomain,
			"dnsPort":    h.cfg.DNSPort,
		})
		return
	}

	expiresAt := user.LastActive.Add(180 * 24 * time.Hour)
	writeJSON(w, http.StatusOK, map[string]any{
		"logged_in":   true,
		"id":          user.ID,
		"subdomain":   user.Subdomain,
		"domain":      fmt.Sprintf("%s.%s", user.Subdomain, h.cfg.BaseDomain),
		"baseDomain":  h.cfg.BaseDomain,
		"dnsPort":     h.cfg.DNSPort,
		"created_at":  user.CreatedAt,
		"last_active": user.LastActive,
		"expires_at":  expiresAt,
	})
}

func (h *APIHandler) handleRenewSession(w http.ResponseWriter, r *http.Request, subdomain string) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	user, err := h.db.RenewUser(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal memperpanjang masa aktif subdomain: "+err.Error())
		return
	}

	expiresAt := user.LastActive.Add(180 * 24 * time.Hour)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"subdomain":   subdomain,
		"last_active": user.LastActive,
		"expires_at":  expiresAt,
		"message":     "Masa aktif subdomain berhasil diperpanjang hingga 6 bulan ke depan.",
	})
}

func (h *APIHandler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	http.SetCookie(w, &http.Cookie{
		Name:    "zcdns_subdomain",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *APIHandler) handleDeleteSubdomain(w http.ResponseWriter, r *http.Request, subdomain string) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.db.DeleteSubdomain(subdomain); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal menghapus subdomain: "+err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "zcdns_subdomain",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})

	h.triggerRecordChanged()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Subdomain dan seluruh record berhasil dihapus.",
	})
}

func (h *APIHandler) handleGetRecords(w http.ResponseWriter, r *http.Request, subdomain string) {
	records, err := h.db.GetRecords(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to retrieve records: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

type RecordRequest struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
}

func (h *APIHandler) handleCreateRecord(w http.ResponseWriter, r *http.Request, subdomain string) {
	// Capacity check (max 9,999 records for slave zone safety)
	totalRecs, err := h.db.GetTotalRecordCount()
	if err == nil && totalRecs >= 9999 {
		writeJSONError(w, http.StatusServiceUnavailable, "ZeroCentDNS saat ini mencapai batas kapasitas maksimum (9.999 record). Mohon menunggu beberapa saat hingga kapasitas tersedia kembali.")
		return
	}

	var req RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	cleanName, cleanType, cleanValue, ttl, err := h.validateRecord(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	record := &db.Record{
		ID:        uuid.New().String(),
		Subdomain: subdomain,
		Name:      cleanName,
		Type:      cleanType,
		Value:     cleanValue,
		TTL:       ttl,
	}

	if err := h.db.AddRecord(record); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to store record: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, record)
	h.triggerRecordChanged()
}

func (h *APIHandler) handleUpdateRecord(w http.ResponseWriter, r *http.Request, subdomain string) {
	recordID := r.PathValue("id")
	if recordID == "" {
		writeJSONError(w, http.StatusBadRequest, "Record ID required")
		return
	}

	var req RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	cleanName, cleanType, cleanValue, ttl, err := h.validateRecord(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	record := &db.Record{
		ID:        recordID,
		Subdomain: subdomain,
		Name:      cleanName,
		Type:      cleanType,
		Value:     cleanValue,
		TTL:       ttl,
	}

	if err := h.db.UpdateRecord(record); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update record: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, record)
	h.triggerRecordChanged()
}

func (h *APIHandler) handleDeleteRecord(w http.ResponseWriter, r *http.Request, subdomain string) {
	recordID := r.PathValue("id")
	if recordID == "" {
		writeJSONError(w, http.StatusBadRequest, "Record ID required")
		return
	}

	if err := h.db.DeleteRecord(subdomain, recordID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete record: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
	h.triggerRecordChanged()
}

func (h *APIHandler) handleDeleteAllRecords(w http.ResponseWriter, r *http.Request, subdomain string) {
	if err := h.db.DeleteAllRecords(subdomain); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to clear records: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
	h.triggerRecordChanged()
}

func (h *APIHandler) handleGetRequests(w http.ResponseWriter, r *http.Request, subdomain string) {
	requests, err := h.db.GetRecentRequests(subdomain, 100)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to retrieve requests: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (h *APIHandler) handleDeleteRequests(w http.ResponseWriter, r *http.Request, subdomain string) {
	if err := h.db.DeleteRequests(subdomain); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete requests: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *APIHandler) handleWebSocketStream(w http.ResponseWriter, r *http.Request) {
	subdomain := h.getSubdomain(r)
	if subdomain == "" {
		http.Error(w, "Subdomain required", http.StatusBadRequest)
		return
	}
	h.hub.HandleWebSocket(w, r, subdomain)
}

func (h *APIHandler) validateRecord(req RecordRequest) (name string, recordType string, value string, ttl int, err error) {
	cleanType := strings.ToUpper(strings.TrimSpace(req.Type))
	validTypes := map[string]bool{
		"A": true, "AAAA": true, "CNAME": true, "TXT": true,
		"MX": true, "NS": true, "PTR": true, "CAA": true, "SRV": true,
	}
	if !validTypes[cleanType] {
		return "", "", "", 0, fmt.Errorf("unsupported record type: %s", cleanType)
	}

	cleanName := strings.TrimSpace(strings.ToLower(req.Name))
	cleanName = strings.TrimSuffix(cleanName, ".")
	if cleanName == "" {
		cleanName = "@"
	}

	cleanValue := strings.TrimSpace(req.Value)
	if cleanValue == "" {
		return "", "", "", 0, fmt.Errorf("record value cannot be empty")
	}

	if cleanType == "TXT" && !strings.HasPrefix(cleanValue, "\"") {
		cleanValue = fmt.Sprintf("\"%s\"", cleanValue)
	}

	ttl = req.TTL
	if ttl <= 0 {
		ttl = 60
	}
	if ttl > 86400 {
		ttl = 86400
	}

	return cleanName, cleanType, cleanValue, ttl, nil
}

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

// Growth stats endpoint
func (h *APIHandler) handleGetGrowthStats(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	stats, err := h.db.GetGrowthStats()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal mengambil statistik: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// Abuse Report Public Submission
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

// Admin Authentication & Handlers
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

type AcmeChallengeRequest struct {
	Subdomain string `json:"subdomain"`
	Value     string `json:"value"`
}

func (h *APIHandler) handleCreateAcmeChallenge(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req AcmeChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
	if sub == "" {
		sub = "guard"
	}
	val := strings.Trim(strings.TrimSpace(req.Value), "\"")
	if val == "" {
		writeJSONError(w, http.StatusBadRequest, "Challenge value is required")
		return
	}

	record := &db.Record{
		ID:        uuid.New().String(),
		Subdomain: sub,
		Name:      "_acme-challenge",
		Type:      "TXT",
		Value:     val,
		TTL:       60,
	}

	if err := h.db.AddRecord(record); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save challenge record: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success":   true,
		"subdomain": sub,
		"name":      "_acme-challenge",
		"value":     val,
	})
	h.triggerRecordChanged()
}

func (h *APIHandler) handleGetAcmeChallenges(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	sub := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("subdomain")))
	if sub == "" {
		sub = "guard"
	}
	records, err := h.db.GetRecordsByNameAndType(sub, "_acme-challenge", "TXT")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *APIHandler) handleAdminGetTXTRecords(w http.ResponseWriter, r *http.Request) {
	records, err := h.db.GetAllTXTRecords()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
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
