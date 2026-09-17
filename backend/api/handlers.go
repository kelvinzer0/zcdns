package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
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
	cfg      *config.Config
	db       *db.DB
	hub      *StreamHub
	parental *parental.Engine
}

func NewAPIHandler(cfg *config.Config, database *db.DB, hub *StreamHub, pe *parental.Engine) *APIHandler {
	return &APIHandler{
		cfg:      cfg,
		db:       database,
		hub:      hub,
		parental: pe,
	}
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// CORS and JSON middleware wrappers
	mux.HandleFunc("POST /api/session", h.handleCreateSession)
	mux.HandleFunc("GET /api/session", h.handleGetSession)
	mux.HandleFunc("DELETE /api/session", h.handleDeleteSession)

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

	mux.HandleFunc("GET /api/requeststream", h.handleWebSocketStream)
	// In addition, support /requeststream/{subdomain} or query param
	mux.HandleFunc("GET /requeststream", h.handleWebSocketStream)
	mux.HandleFunc("GET /requeststream/{subdomain}", h.handleWebSocketStream)

	mux.HandleFunc("POST /api/test-query", h.handleTestQuery)

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
	// 1. Path value (if route has {subdomain})
	if sub := r.PathValue("subdomain"); sub != "" {
		return strings.ToLower(sub)
	}
	// 2. Query param
	if sub := r.URL.Query().Get("subdomain"); sub != "" {
		return strings.ToLower(sub)
	}
	// 3. Header
	if sub := r.Header.Get("X-Subdomain"); sub != "" {
		return strings.ToLower(sub)
	}
	// 4. Cookie
	if c, err := r.Cookie("zcdns_subdomain"); err == nil && c.Value != "" {
		return strings.ToLower(c.Value)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"logged_in":  true,
		"id":         user.ID,
		"subdomain":  user.Subdomain,
		"domain":     fmt.Sprintf("%s.%s", user.Subdomain, h.cfg.BaseDomain),
		"baseDomain": h.cfg.BaseDomain,
		"dnsPort":    h.cfg.DNSPort,
		"created_at": user.CreatedAt,
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
}

func (h *APIHandler) handleDeleteAllRecords(w http.ResponseWriter, r *http.Request, subdomain string) {
	if err := h.db.DeleteAllRecords(subdomain); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to clear records: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
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
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Subdomain, Authorization")
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
