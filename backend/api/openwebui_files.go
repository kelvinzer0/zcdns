package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"zcdns-backend/db"
)

// getUploadsDir ensures and returns the uploads directory for a subdomain
func (h *APIHandler) getUploadsDir(subdomain string) string {
	baseDir := "data/uploads"
	if h.cfg != nil && h.cfg.DBPath != "" {
		baseDir = filepath.Join(filepath.Dir(h.cfg.DBPath), "uploads")
	}
	dir := filepath.Join(baseDir, subdomain)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

// isTextData determines if a byte slice is text-based (UTF-8 without null bytes)
func isTextData(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	checkLen := len(data)
	if checkLen > 8192 {
		checkLen = 8192
	}
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return false
		}
	}
	return utf8.Valid(data[:checkLen])
}

// formatFileModel maps a DB file record to OpenWebUI's expected FileModelResponse
func formatFileModel(f db.OpenWebUIFileDB, includeContent bool) map[string]interface{} {
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(f.MetaJSON), &meta); err != nil || meta == nil {
		meta = map[string]interface{}{}
	}
	meta["name"] = f.Filename
	meta["content_type"] = f.ContentType
	meta["size"] = f.Size
	if f.Hash != "" {
		meta["file_hash"] = f.Hash
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(f.DataJSON), &data); err != nil || data == nil {
		data = map[string]interface{}{}
	}
	if !includeContent {
		delete(data, "content")
	}

	res := map[string]interface{}{
		"id":         f.ID,
		"user_id":    f.UserID,
		"filename":   f.Filename,
		"meta":       meta,
		"data":       data,
		"created_at": f.CreatedAt,
		"updated_at": f.UpdatedAt,
		"status":     true,
	}
	if f.Hash != "" {
		res["hash"] = f.Hash
	}
	return res
}

// handleOpenWebUIFiles handles /api/v1/files/*
func (h *APIHandler) handleOpenWebUIFiles(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}
	subdomain := h.resolveSubdomain(r)
	u := h.resolveUser(r)

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/files")
	path = strings.TrimPrefix(path, "/")

	// 1. POST /api/v1/files/ (Upload file)
	if (path == "" || path == "/") && r.Method == http.MethodPost {
		// Limit to 100MB
		err := r.ParseMultipartForm(100 << 20)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Failed to parse multipart upload: "+err.Error())
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Missing 'file' field in form")
			return
		}
		defer file.Close()

		contents, err := io.ReadAll(file)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to read file contents: "+err.Error())
			return
		}

		// Parse metadata if provided
		var metaData map[string]interface{}
		metaStr := r.FormValue("metadata")
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &metaData)
		}
		if metaData == nil {
			metaData = map[string]interface{}{}
		}

		// Calculate SHA-256
		hashBytes := sha256.Sum256(contents)
		fileHash := hex.EncodeToString(hashBytes[:])
		if h, ok := metaData["file_hash"].(string); ok && h != "" {
			fileHash = h
		}

		// Generate random ID
		randBytes := make([]byte, 16)
		_, _ = rand.Read(randBytes)
		fileID := hex.EncodeToString(randBytes)

		// Sanitize filename
		origName := filepath.Base(header.Filename)
		if origName == "" || origName == "." || origName == "/" {
			origName = "file_" + fileID[:8]
		}

		// Determine content type
		contentType := header.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = http.DetectContentType(contents)
		}

		// Save file to storage
		storageDir := h.getUploadsDir(subdomain)
		diskFilename := fmt.Sprintf("%s_%s", fileID, origName)
		diskPath := filepath.Join(storageDir, diskFilename)
		_ = os.WriteFile(diskPath, contents, 0644)

		// Extract content if text / document
		ext := strings.ToLower(filepath.Ext(origName))
		textExts := map[string]bool{
			".txt": true, ".md": true, ".markdown": true, ".json": true,
			".csv": true, ".tsv": true, ".html": true, ".css": true,
			".js": true, ".ts": true, ".jsx": true, ".tsx": true,
			".py": true, ".go": true, ".rs": true, ".c": true, ".cpp": true,
			".h": true, ".sh": true, ".bash": true, ".yaml": true,
			".yml": true, ".xml": true, ".sql": true, ".log": true,
			".env": true, ".ini": true, ".conf": true, ".svelte": true,
		}

		var textContent string
		if textExts[ext] || strings.HasPrefix(contentType, "text/") || contentType == "application/json" || isTextData(contents) {
			textContent = string(contents)
		}

		dataMap := map[string]interface{}{
			"status": "completed",
		}
		if textContent != "" {
			dataMap["content"] = textContent
		}
		dataJSON, _ := json.Marshal(dataMap)

		metaMap := map[string]interface{}{
			"name":         origName,
			"content_type": contentType,
			"size":         len(contents),
			"file_hash":    fileHash,
			"data":         metaData,
		}
		metaJSON, _ := json.Marshal(metaMap)

		now := time.Now().Unix()
		fRecord := db.OpenWebUIFileDB{
			ID:          fileID,
			Subdomain:   subdomain,
			UserID:      u.ID,
			Hash:        fileHash,
			Filename:    origName,
			Path:        diskPath,
			ContentType: contentType,
			Size:        int64(len(contents)),
			DataJSON:    string(dataJSON),
			MetaJSON:    string(metaJSON),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := h.db.InsertOpenWebUIFile(fRecord); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to save file metadata: "+err.Error())
			return
		}

		writeJSON(w, http.StatusOK, formatFileModel(fRecord, true))
		return
	}

	// 2. GET /api/v1/files/ (List files)
	if (path == "" || path == "/") && r.Method == http.MethodGet {
		page := 1
		if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p >= 1 {
			page = p
		}
		includeContent := r.URL.Query().Get("content") != "false"
		limit := 50
		skip := (page - 1) * limit

		files, total, err := h.db.GetOpenWebUIFiles(subdomain, u.ID, skip, limit)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"items": []any{}, "total": 0})
			return
		}

		var items []map[string]interface{}
		for _, f := range files {
			items = append(items, formatFileModel(f, includeContent))
		}
		if items == nil {
			items = []map[string]interface{}{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items": items,
			"total": total,
		})
		return
	}

	// 3. GET /api/v1/files/search
	if path == "search" && r.Method == http.MethodGet {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			filename = "*"
		}
		skip := 0
		if s, err := strconv.Atoi(r.URL.Query().Get("skip")); err == nil && s >= 0 {
			skip = s
		}
		limit := 50
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l >= 1 {
			limit = l
		}
		includeContent := r.URL.Query().Get("content") != "false"

		files, err := h.db.SearchOpenWebUIFiles(subdomain, u.ID, filename, skip, limit)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}

		var items []map[string]interface{}
		for _, f := range files {
			items = append(items, formatFileModel(f, includeContent))
		}
		if items == nil {
			items = []map[string]interface{}{}
		}

		writeJSON(w, http.StatusOK, items)
		return
	}

	// 4. GET /api/v1/files/count
	if path == "count" && r.Method == http.MethodGet {
		count, _ := h.db.CountOpenWebUIFiles(subdomain, u.ID)
		writeJSON(w, http.StatusOK, count)
		return
	}

	// 5. DELETE /api/v1/files/all
	if path == "all" && r.Method == http.MethodDelete {
		_ = h.db.DeleteAllOpenWebUIFiles(subdomain, u.ID)
		writeJSON(w, http.StatusOK, map[string]string{"message": "All files deleted successfully"})
		return
	}

	// 6. Sub-paths on /api/v1/files/{id}/*
	parts := strings.Split(path, "/")
	fileID := parts[0]

	if len(parts) == 1 {
		// GET /api/v1/files/{id}
		if r.Method == http.MethodGet {
			file, err := h.db.GetOpenWebUIFileByID(subdomain, fileID)
			if err != nil || file == nil {
				writeJSONError(w, http.StatusNotFound, "File not found")
				return
			}
			writeJSON(w, http.StatusOK, formatFileModel(*file, true))
			return
		}

		// DELETE /api/v1/files/{id}
		if r.Method == http.MethodDelete {
			file, _ := h.db.GetOpenWebUIFileByID(subdomain, fileID)
			if file != nil && file.Path != "" {
				_ = os.Remove(file.Path)
			}
			_ = h.db.DeleteOpenWebUIFile(subdomain, fileID)
			writeJSON(w, http.StatusOK, map[string]string{"message": "File deleted successfully"})
			return
		}
	}

	if len(parts) >= 2 {
		action := parts[1]

		// GET /api/v1/files/{id}/process/status
		if action == "process" && len(parts) >= 3 && parts[2] == "status" {
			if r.URL.Query().Get("stream") == "true" {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("data: {\"status\": \"completed\"}\n\n"))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
			return
		}

		// /api/v1/files/{id}/data/content and /update
		if action == "data" && len(parts) >= 3 && parts[2] == "content" {
			file, err := h.db.GetOpenWebUIFileByID(subdomain, fileID)
			if err != nil || file == nil {
				writeJSONError(w, http.StatusNotFound, "File not found")
				return
			}

			if len(parts) == 3 && r.Method == http.MethodGet {
				var data map[string]interface{}
				_ = json.Unmarshal([]byte(file.DataJSON), &data)
				content, _ := data["content"].(string)
				writeJSON(w, http.StatusOK, map[string]string{"content": content})
				return
			}

			if len(parts) == 4 && parts[3] == "update" && r.Method == http.MethodPost {
				body, _ := io.ReadAll(r.Body)
				var form struct {
					Content string `json:"content"`
				}
				_ = json.Unmarshal(body, &form)

				var data map[string]interface{}
				_ = json.Unmarshal([]byte(file.DataJSON), &data)
				if data == nil {
					data = map[string]interface{}{}
				}
				data["content"] = form.Content
				newDataJSON, _ := json.Marshal(data)
				_ = h.db.UpdateOpenWebUIFileData(subdomain, fileID, string(newDataJSON))

				writeJSON(w, http.StatusOK, map[string]string{"content": form.Content})
				return
			}
		}

		// GET /api/v1/files/{id}/content or /api/v1/files/{id}/content/{filename}
		if action == "content" {
			file, err := h.db.GetOpenWebUIFileByID(subdomain, fileID)
			if err != nil || file == nil {
				writeJSONError(w, http.StatusNotFound, "File not found")
				return
			}

			if file.Path != "" {
				if _, err := os.Stat(file.Path); err == nil {
					filename := file.Filename
					encodedFilename := url.PathEscape(filename)
					attachment := r.URL.Query().Get("attachment") == "true"

					if attachment {
						w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFilename))
					} else if file.ContentType == "application/pdf" || strings.HasSuffix(strings.ToLower(filename), ".pdf") {
						w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", encodedFilename))
						w.Header().Set("Content-Type", "application/pdf")
					} else if strings.HasPrefix(file.ContentType, "image/") {
						w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", encodedFilename))
						w.Header().Set("Content-Type", file.ContentType)
					} else {
						w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFilename))
					}

					http.ServeFile(w, r, file.Path)
					return
				}
			}

			// Fallback: return data.content as text plain if disk file was deleted/empty
			var data map[string]interface{}
			_ = json.Unmarshal([]byte(file.DataJSON), &data)
			content, _ := data["content"].(string)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(content))
			return
		}

		// POST /api/v1/files/{id}/rename
		if action == "rename" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var form struct {
				Filename string `json:"filename"`
			}
			_ = json.Unmarshal(body, &form)
			if form.Filename != "" {
				_ = h.db.UpdateOpenWebUIFileName(subdomain, fileID, form.Filename)
			}
			file, _ := h.db.GetOpenWebUIFileByID(subdomain, fileID)
			if file != nil {
				writeJSON(w, http.StatusOK, formatFileModel(*file, true))
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"filename": form.Filename})
			return
		}
	}

	writeJSONError(w, http.StatusNotFound, "Endpoint not found")
}
