package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zcdns-backend/config"
	"zcdns-backend/db"
)

func setupTestHandler(t *testing.T) (*APIHandler, *db.DB, func()) {
	tmpDir, err := os.MkdirTemp("", "zcdns_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init db: %v", err)
	}

	cfg := &config.Config{
		DBPath:    dbPath,
		StaticDir: tmpDir,
	}

	handler := NewAPIHandler(cfg, database, nil, nil)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return handler, database, cleanup
}

func TestFileUploadAndRetrieval(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Upload a text document
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "notes.md")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	content := "# My Meeting Notes\n\nDiscussing the OpenWebUI architecture."
	_, _ = io.WriteString(part, content)
	_ = writer.WriteField("metadata", `{"category": "notes", "tags": ["work"]}`)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-token-123")
	rec := httptest.NewRecorder()

	handler.handleOpenWebUIFiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var uploadResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to unmarshal upload response: %v", err)
	}

	fileID, ok := uploadResp["id"].(string)
	if !ok || fileID == "" {
		t.Fatalf("expected valid file id, got %v", uploadResp["id"])
	}

	dataMap, ok := uploadResp["data"].(map[string]interface{})
	if !ok || dataMap["content"] != content {
		t.Fatalf("expected extracted content %q, got %v", content, dataMap["content"])
	}

	// 2. Test GET /api/v1/files/{id}/process/status (stream=true)
	reqStatus := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/process/status?stream=true", fileID), nil)
	reqStatus.Header.Set("Authorization", "Bearer test-token-123")
	recStatus := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recStatus, reqStatus)

	if recStatus.Code != http.StatusOK {
		t.Fatalf("expected status 200 for process status, got %d", recStatus.Code)
	}
	if !strings.Contains(recStatus.Body.String(), `"status": "completed"`) {
		t.Fatalf("expected completed status stream, got %s", recStatus.Body.String())
	}

	// 3. Test GET /api/v1/files/{id}/data/content
	reqData := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/data/content", fileID), nil)
	reqData.Header.Set("Authorization", "Bearer test-token-123")
	recData := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recData, reqData)

	var dataContentResp map[string]string
	_ = json.Unmarshal(recData.Body.Bytes(), &dataContentResp)
	if dataContentResp["content"] != content {
		t.Fatalf("expected data content %q, got %q", content, dataContentResp["content"])
	}

	// 4. Test POST /api/v1/files/{id}/data/content/update
	newContent := "# Updated Notes\n\nAll items resolved."
	updateBody := strings.NewReader(fmt.Sprintf(`{"content": %q}`, newContent))
	reqUpdate := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/files/%s/data/content/update", fileID), updateBody)
	reqUpdate.Header.Set("Authorization", "Bearer test-token-123")
	reqUpdate.Header.Set("Content-Type", "application/json")
	recUpdate := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 on content update, got %d", recUpdate.Code)
	}

	// Verify update in data/content
	reqVerify := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/data/content", fileID), nil)
	reqVerify.Header.Set("Authorization", "Bearer test-token-123")
	recVerify := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recVerify, reqVerify)
	_ = json.Unmarshal(recVerify.Body.Bytes(), &dataContentResp)
	if dataContentResp["content"] != newContent {
		t.Fatalf("expected updated content %q, got %q", newContent, dataContentResp["content"])
	}

	// 5. Test GET /api/v1/files/{id}/content
	reqContent := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/files/%s/content", fileID), nil)
	reqContent.Header.Set("Authorization", "Bearer test-token-123")
	recContent := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recContent, reqContent)

	if recContent.Code != http.StatusOK {
		t.Fatalf("expected status 200 for binary content, got %d", recContent.Code)
	}
	if recContent.Body.String() != content {
		t.Fatalf("expected raw binary content %q, got %q", content, recContent.Body.String())
	}

	// 6. Test GET /api/v1/files/search
	reqSearch := httptest.NewRequest(http.MethodGet, "/api/v1/files/search?filename=*notes*", nil)
	reqSearch.Header.Set("Authorization", "Bearer test-token-123")
	recSearch := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recSearch, reqSearch)

	var searchList []map[string]interface{}
	_ = json.Unmarshal(recSearch.Body.Bytes(), &searchList)
	if len(searchList) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(searchList))
	}

	// 7. Test GET /api/v1/files/count
	reqCount := httptest.NewRequest(http.MethodGet, "/api/v1/files/count", nil)
	reqCount.Header.Set("Authorization", "Bearer test-token-123")
	recCount := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recCount, reqCount)

	var count int
	_ = json.Unmarshal(recCount.Body.Bytes(), &count)
	if count != 1 {
		t.Fatalf("expected file count 1, got %d", count)
	}

	// 8. Test DELETE /api/v1/files/{id}
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/files/%s", fileID), nil)
	reqDel.Header.Set("Authorization", "Bearer test-token-123")
	recDel := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", recDel.Code)
	}

	// Count after delete should be 0
	recCount2 := httptest.NewRecorder()
	handler.handleOpenWebUIFiles(recCount2, reqCount)
	_ = json.Unmarshal(recCount2.Body.Bytes(), &count)
	if count != 0 {
		t.Fatalf("expected file count 0 after deletion, got %d", count)
	}
}

func TestUserProfileAndAvatar(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Update user profile via POST /api/v1/auths/update/profile
	samplePNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
	base64Avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(samplePNG)

	updatePayload := fmt.Sprintf(`{
		"name": "Jane Doe",
		"profile_image_url": %q,
		"bio": "AI Researcher & Builder",
		"gender": "female",
		"date_of_birth": "1995-05-15"
	}`, base64Avatar)

	reqUpdate := httptest.NewRequest(http.MethodPost, "/api/v1/auths/update/profile", strings.NewReader(updatePayload))
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate.Header.Set("Authorization", "Bearer user-token-abc")
	recUpdate := httptest.NewRecorder()

	handler.handleOpenWebUIAuth(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 on profile update, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	var updateResp map[string]interface{}
	_ = json.Unmarshal(recUpdate.Body.Bytes(), &updateResp)
	if updateResp["name"] != "Jane Doe" {
		t.Fatalf("expected name 'Jane Doe', got %v", updateResp["name"])
	}
	if updateResp["profile_image_url"] != base64Avatar {
		t.Fatalf("expected profile_image_url to match base64 avatar")
	}

	// 2. Fetch session user (GET /api/v1/auths) and verify persisted profile attributes
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/auths", nil)
	reqAuth.Header.Set("Authorization", "Bearer user-token-abc")
	recAuth := httptest.NewRecorder()

	handler.handleOpenWebUIAuth(recAuth, reqAuth)

	var sessionUser OpenWebUISessionUserInfoResponse
	if err := json.Unmarshal(recAuth.Body.Bytes(), &sessionUser); err != nil {
		t.Fatalf("failed to decode session user: %v", err)
	}

	if sessionUser.Name != "Jane Doe" {
		t.Fatalf("expected session user name 'Jane Doe', got %s", sessionUser.Name)
	}
	if sessionUser.ProfileImageURL != base64Avatar {
		t.Fatalf("expected session user avatar to be preserved")
	}
	if sessionUser.Bio == nil || *sessionUser.Bio != "AI Researcher & Builder" {
		t.Fatalf("expected bio to be preserved")
	}

	// 3. Serve binary avatar via GET /api/v1/users/user/profile/image
	reqImg := httptest.NewRequest(http.MethodGet, "/api/v1/users/user/profile/image", nil)
	reqImg.Header.Set("Authorization", "Bearer user-token-abc")
	recImg := httptest.NewRecorder()

	handler.handleOpenWebUIUsers(recImg, reqImg)

	if recImg.Code != http.StatusOK {
		t.Fatalf("expected 200 for binary avatar, got %d: %s", recImg.Code, recImg.Body.String())
	}
	if recImg.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %s", recImg.Header().Get("Content-Type"))
	}
	if !bytes.Equal(recImg.Body.Bytes(), samplePNG) {
		t.Fatalf("expected decoded avatar bytes to match samplePNG")
	}
}
