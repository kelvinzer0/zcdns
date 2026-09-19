package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zcdns-backend/config"
	"zcdns-backend/db"
	"zcdns-backend/parental"

	"github.com/google/uuid"
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

	mux.HandleFunc("POST /api/vault/init", h.requireSubdomain(h.handleInitVault))
	mux.HandleFunc("GET /api/vault/branches", h.requireSubdomain(h.handleGetVaultBranches))
	mux.HandleFunc("DELETE /api/vault/branches", h.requireSubdomain(h.handleDeleteVaultBranch))
	mux.HandleFunc("GET /api/vault/commits", h.requireSubdomain(h.handleGetVaultCommits))
	mux.HandleFunc("GET /api/vault/kv", h.requireSubdomain(h.handleGetVaultKV))
	mux.HandleFunc("POST /api/vault/revert", h.requireSubdomain(h.handleRevertVaultCommit))
	mux.HandleFunc("POST /api/vault/sync", h.requireSubdomain(h.handleSyncVault))

	mux.HandleFunc("POST /api/vault/auth/device", h.handleVaultDeviceAuth)
	mux.HandleFunc("POST /api/vault/auth/poll", h.handleVaultPollAuth)
	mux.HandleFunc("GET /api/vault/auth/info", h.handleVaultGetAuthInfo)
	mux.HandleFunc("POST /api/vault/auth/verify", h.requireSubdomain(h.handleVaultVerifyAuth))

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
	mux.HandleFunc("POST /api/acme-challenge", h.requireAdmin(h.handleCreateAcmeChallenge))
	mux.HandleFunc("GET /api/acme-challenge", h.requireAdmin(h.handleGetAcmeChallenges))

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

// Admin Authentication & Handlers

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
