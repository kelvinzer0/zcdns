package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zcdns-backend/db"

	"github.com/google/uuid"
)

func generateUserCode() string {
	const charset = "BCDFGHJKLMNPQRSTVWXYZ23456789"
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	result := make([]byte, 8)
	for i, b := range bytes {
		result[i] = charset[int(b)%len(charset)]
	}
	return fmt.Sprintf("%s-%s", string(result[:4]), string(result[4:]))
}

func generateVaultToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "zvt_" + hex.EncodeToString(b)
}

func (h *APIHandler) handleInitVault(w http.ResponseWriter, r *http.Request, subdomain string) {
	repo, err := h.db.InitVault(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to init vault: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *APIHandler) handleGetVaultBranches(w http.ResponseWriter, r *http.Request, subdomain string) {
	repoID := r.URL.Query().Get("repo_id")
	if repoID == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_id is required")
		return
	}
	branches, err := h.db.GetBranches(repoID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get branches: "+err.Error())
		return
	}
	if branches == nil {
		branches = make([]db.VaultBranch, 0)
	}
	writeJSON(w, http.StatusOK, branches)
}

func (h *APIHandler) handleGetVaultCommits(w http.ResponseWriter, r *http.Request, subdomain string) {
	repoID := r.URL.Query().Get("repo_id")
	if repoID == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_id is required")
		return
	}
	commits, err := h.db.GetCommits(repoID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get commits: "+err.Error())
		return
	}
	if commits == nil {
		commits = make([]db.VaultCommit, 0)
	}
	writeJSON(w, http.StatusOK, commits)
}

func (h *APIHandler) handleGetVaultKV(w http.ResponseWriter, r *http.Request, subdomain string) {
	commit := r.URL.Query().Get("commit")
	if commit == "" {
		writeJSONError(w, http.StatusBadRequest, "commit is required")
		return
	}
	kv, err := h.db.GetVaultKVPairs(commit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get KV pairs: "+err.Error())
		return
	}
	if kv == nil {
		kv = make([]db.VaultKVPair, 0)
	}
	writeJSON(w, http.StatusOK, kv)
}

func (h *APIHandler) handleRevertVaultCommit(w http.ResponseWriter, r *http.Request, subdomain string) {
	var req struct {
		RepoID     string `json:"repo_id"`
		CommitHash string `json:"commit_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.RepoID == "" || req.CommitHash == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_id and commit_hash are required")
		return
	}
	// The user meant "revert commit harus nya setelah itu hilang commit yang terbaru"
	// Let's just delete the specific commit. If it's the latest, it will rollback.
	if err := h.db.DeleteCommit(req.RepoID, req.CommitHash); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to revert commit: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Device Auth Flow (RFC 8628 style for zvault CLI)

func (h *APIHandler) handleVaultDeviceAuth(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	deviceCode := uuid.New().String()
	userCode := generateUserCode()
	expiresAt := time.Now().Add(10 * time.Minute)

	req := &db.VaultAuthRequest{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Status:     "pending",
		ExpiresAt:  expiresAt,
	}

	if err := h.db.CreateVaultAuthRequest(req); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create device auth: "+err.Error())
		return
	}

	host := r.Host
	if host == "" {
		host = "zcdns.id"
	}
	// normalize host for clean display
	if !strings.Contains(host, "zcdns.id") && !strings.Contains(host, "localhost") {
		host = "zcdns.id"
	}

	scheme := "https"
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		scheme = "http"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               deviceCode,
		"user_code":                 userCode,
		"verification_uri":          fmt.Sprintf("%s://%s/vault/auth", scheme, host),
		"verification_uri_complete": fmt.Sprintf("%s://%s/vault/auth?code=%s", scheme, host, userCode),
		"expires_in":                600,
		"interval":                  2,
	})
}

type PollRequest struct {
	DeviceCode string `json:"device_code"`
}

func (h *APIHandler) handleVaultPollAuth(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req PollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceCode == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	authReq, err := h.db.GetVaultAuthByDeviceCode(req.DeviceCode)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Authentication request not found")
		return
	}

	if time.Now().After(authReq.ExpiresAt) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "expired",
			"error":  "expired_token",
		})
		return
	}

	if authReq.Status == "approved" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "approved",
			"token":     authReq.Token,
			"subdomain": authReq.Subdomain,
		})
		return
	}

	if authReq.Status == "denied" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "denied",
			"error":  "access_denied",
		})
		return
	}

	// Still pending
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "pending",
	})
}

func (h *APIHandler) handleVaultGetAuthInfo(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	code := strings.TrimSpace(strings.ToUpper(r.URL.Query().Get("code")))
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "code is required")
		return
	}

	authReq, err := h.db.GetVaultAuthByUserCode(code)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Kode otentikasi tidak ditemukan")
		return
	}

	isExpired := time.Now().After(authReq.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_code":  authReq.UserCode,
		"status":     authReq.Status,
		"is_expired": isExpired,
		"subdomain":  authReq.Subdomain,
	})
}

type VerifyRequest struct {
	UserCode  string `json:"user_code"`
	Action    string `json:"action"` // "approve" or "deny"
	Subdomain string `json:"subdomain"`
}

func (h *APIHandler) handleVaultVerifyAuth(w http.ResponseWriter, r *http.Request, sessionSubdomain string) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.UserCode))
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "user_code is required")
		return
	}

	authReq, err := h.db.GetVaultAuthByUserCode(code)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Otentikasi tidak ditemukan")
		return
	}

	if time.Now().After(authReq.ExpiresAt) {
		writeJSONError(w, http.StatusBadRequest, "Kode otentikasi telah kadaluarsa")
		return
	}

	if authReq.Status != "pending" {
		writeJSONError(w, http.StatusBadRequest, "Kode ini sudah diproses sebelumnya ("+authReq.Status+")")
		return
	}

	if req.Action == "deny" {
		_ = h.db.DenyVaultAuth(code)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"status":  "denied",
		})
		return
	}

	sub := strings.TrimSpace(strings.ToLower(sessionSubdomain))
	if sub == "" {
		writeJSONError(w, http.StatusUnauthorized, "Sesi login tidak valid")
		return
	}

	token := generateVaultToken()
	if err := h.db.ApproveVaultAuth(code, sub, token); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal menyetujui otentikasi: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"status":    "approved",
		"subdomain": sub,
	})
}

type SyncVaultRequest struct {
	RepoID         string           `json:"repo_id"`
	Commits        []db.VaultCommit `json:"commits"`
	KVPairs        []db.VaultKVPair `json:"kv_pairs"`
	HeadCommitHash string           `json:"head_commit_hash"`
}

func (h *APIHandler) handleSyncVault(w http.ResponseWriter, r *http.Request, subdomain string) {
	var req SyncVaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if req.RepoID == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_id is required")
		return
	}
	
	if err := h.db.SyncVault(req.RepoID, req.Commits, req.KVPairs, req.HeadCommitHash); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to sync vault: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
