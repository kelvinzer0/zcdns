package api

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"
	"zcdns-backend/db"

	"github.com/google/uuid"
)

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
