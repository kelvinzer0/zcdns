package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"zcdns-backend/db"

	"github.com/google/uuid"
)

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
