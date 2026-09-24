package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenWebUIPromptsCRUD(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Create a prompt
	createPayload := map[string]string{
		"command": "/summarize",
		"name":    "Summarize Text",
		"content": "Please summarize the following text: {{TEXT}}",
	}
	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompts/create", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token-1")
	rec := httptest.NewRecorder()
	handler.handleOpenWebUIPrompts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on create, got %d: %s", rec.Code, rec.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created prompt: %v", err)
	}
	if created["command"] != "summarize" || created["name"] != "Summarize Text" {
		t.Fatalf("unexpected prompt data: %v", created)
	}
	pID, ok := created["id"].(string)
	if !ok || pID == "" {
		t.Fatalf("expected created prompt to have id, got %v", created["id"])
	}
	if created["write_access"] != true {
		t.Fatalf("expected write_access=true, got %v", created["write_access"])
	}

	// 2. List prompts
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/list", nil)
	reqList.Header.Set("Authorization", "Bearer test-token-1")
	recList := httptest.NewRecorder()
	handler.handleOpenWebUIPrompts(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d: %s", recList.Code, recList.Body.String())
	}
	var listResp OpenWebUIPaginatedListResponse
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 prompt, got total=%d items=%d", listResp.Total, len(listResp.Items))
	}

	// 3. Get prompt by ID
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/id/"+pID, nil)
	reqGet.Header.Set("Authorization", "Bearer test-token-1")
	recGet := httptest.NewRecorder()
	handler.handleOpenWebUIPrompts(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get by id, got %d: %s", recGet.Code, recGet.Body.String())
	}

	// 4. Update prompt by ID
	updatePayload := map[string]string{
		"command": "/summarize2",
		"name":    "Summarize Updated",
		"content": "Updated content: {{TEXT}}",
	}
	upBody, _ := json.Marshal(updatePayload)
	reqUp := httptest.NewRequest(http.MethodPost, "/api/v1/prompts/id/"+pID+"/update", bytes.NewReader(upBody))
	reqUp.Header.Set("Authorization", "Bearer test-token-1")
	recUp := httptest.NewRecorder()
	handler.handleOpenWebUIPrompts(recUp, reqUp)

	if recUp.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", recUp.Code, recUp.Body.String())
	}

	// 5. Delete prompt by ID
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/prompts/id/"+pID+"/delete", nil)
	reqDel.Header.Set("Authorization", "Bearer test-token-1")
	recDel := httptest.NewRecorder()
	handler.handleOpenWebUIPrompts(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// Verify deletion
	recVerify := httptest.NewRecorder()
	handler.handleOpenWebUIPrompts(recVerify, reqGet)
	if recVerify.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after deletion, got %d", recVerify.Code)
	}
}

func TestOpenWebUIKnowledgeCRUD(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Create a knowledge base
	createPayload := map[string]any{
		"name":          "Engineering Docs",
		"description":   "Internal API and architecture notes",
		"access_grants": []any{},
	}
	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/create", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token-1")
	rec := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on create, got %d: %s", rec.Code, rec.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created knowledge: %v", err)
	}
	kID, ok := created["id"].(string)
	if !ok || kID == "" {
		t.Fatalf("expected created knowledge to have id, got %v", created["id"])
	}
	if created["name"] != "Engineering Docs" {
		t.Fatalf("unexpected name: %v", created["name"])
	}
	if created["write_access"] != true {
		t.Fatalf("expected write_access=true, got %v", created["write_access"])
	}

	// 2. List knowledge bases
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/", nil)
	reqList.Header.Set("Authorization", "Bearer test-token-1")
	recList := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d: %s", recList.Code, recList.Body.String())
	}
	var listResp OpenWebUIPaginatedListResponse
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 knowledge base, got total=%d items=%d", listResp.Total, len(listResp.Items))
	}

	// 3. Get knowledge base by ID
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/"+kID, nil)
	reqGet.Header.Set("Authorization", "Bearer test-token-1")
	recGet := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get by id, got %d: %s", recGet.Code, recGet.Body.String())
	}

	// 4. Update knowledge base
	newDesc := "Updated architecture notes"
	updatePayload := map[string]any{
		"name":        "Engineering Docs v2",
		"description": newDesc,
	}
	upBody, _ := json.Marshal(updatePayload)
	reqUp := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/"+kID+"/update", bytes.NewReader(upBody))
	reqUp.Header.Set("Authorization", "Bearer test-token-1")
	recUp := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recUp, reqUp)

	if recUp.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", recUp.Code, recUp.Body.String())
	}

	// 5. Delete knowledge base
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/knowledge/"+kID+"/delete", nil)
	reqDel.Header.Set("Authorization", "Bearer test-token-1")
	recDel := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// Verify deletion
	recVerify := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recVerify, reqGet)
	if recVerify.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after deletion, got %d", recVerify.Code)
	}
}

func TestOpenWebUIModelsCRUD(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Create custom model
	createPayload := map[string]any{
		"id":            "custom-analyst",
		"name":          "Data Analyst AI",
		"base_model_id": "gpt-4o",
		"params": map[string]any{
			"temperature": 0.2,
		},
		"meta": map[string]any{
			"description": "Specialized in data analytics",
		},
		"access_grants": []any{},
		"is_active":     true,
	}
	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/models/create", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token-1")
	rec := httptest.NewRecorder()
	handler.handleOpenWebUIModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on create model, got %d: %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created model: %v", err)
	}
	if created["id"] != "custom-analyst" || created["write_access"] != true {
		t.Fatalf("unexpected model data: %v", created)
	}

	// 2. List custom models in workspace (/api/v1/models/list)
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/models/list", nil)
	reqList.Header.Set("Authorization", "Bearer test-token-1")
	recList := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 on models list, got %d: %s", recList.Code, recList.Body.String())
	}
	var listResp struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 custom model, got total=%d items=%d", listResp.Total, len(listResp.Items))
	}
	if listResp.Items[0]["write_access"] != true {
		t.Fatalf("expected write_access=true on list item, got %v", listResp.Items[0]["write_access"])
	}

	// 3. Get custom model details (/api/v1/models/model?id=custom-analyst)
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/models/model?id=custom-analyst", nil)
	reqGet.Header.Set("Authorization", "Bearer test-token-1")
	recGet := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get model, got %d: %s", recGet.Code, recGet.Body.String())
	}
	var modelResp map[string]any
	if err := json.Unmarshal(recGet.Body.Bytes(), &modelResp); err != nil {
		t.Fatalf("failed to decode model response: %v", err)
	}
	if modelResp["id"] != "custom-analyst" || modelResp["write_access"] != true {
		t.Fatalf("unexpected get model data: %v", modelResp)
	}

	// 4. Custom model should show up in main /api/models list
	reqAll := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	recAll := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recAll, reqAll)

	if recAll.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/models, got %d: %s", recAll.Code, recAll.Body.String())
	}
	var allResp struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(recAll.Body.Bytes(), &allResp)
	found := false
	for _, m := range allResp.Data {
		if m["id"] == "custom-analyst" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected custom-analyst to appear in /api/models data: %v", allResp.Data)
	}

	// 5. Toggle model active state
	reqToggle := httptest.NewRequest(http.MethodPost, "/api/v1/models/model/toggle?id=custom-analyst", nil)
	reqToggle.Header.Set("Authorization", "Bearer test-token-1")
	recToggle := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recToggle, reqToggle)

	if recToggle.Code != http.StatusOK {
		t.Fatalf("expected 200 on toggle model, got %d: %s", recToggle.Code, recToggle.Body.String())
	}
	var toggled map[string]any
	_ = json.Unmarshal(recToggle.Body.Bytes(), &toggled)
	if toggled["is_active"] != false {
		t.Fatalf("expected is_active=false after toggle, got %v", toggled["is_active"])
	}

	// 6. Delete custom model
	delBody, _ := json.Marshal(map[string]string{"id": "custom-analyst"})
	reqDel := httptest.NewRequest(http.MethodPost, "/api/v1/models/model/delete", bytes.NewReader(delBody))
	reqDel.Header.Set("Authorization", "Bearer test-token-1")
	recDel := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete model, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// Verify deletion from list
	recList2 := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recList2, reqList)
	var listResp2 struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	_ = json.Unmarshal(recList2.Body.Bytes(), &listResp2)
	if listResp2.Total != 0 || len(listResp2.Items) != 0 {
		t.Fatalf("expected 0 models after deletion, got %d", listResp2.Total)
	}
}

func TestOpenWebUIUserStatus(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	token := "status-test-token"

	// 1. Update user status
	statusPayload := map[string]any{
		"status_emoji":   "rocket",
		"status_message": "Working on AI features",
	}
	body, _ := json.Marshal(statusPayload)
	reqUpdate := httptest.NewRequest(http.MethodPost, "/api/v1/users/user/status/update", bytes.NewReader(body))
	reqUpdate.Header.Set("Authorization", "Bearer "+token)
	recUpdate := httptest.NewRecorder()
	handler.handleOpenWebUIUsers(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 on status update, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	var updatedUser OpenWebUISessionUserInfoResponse
	if err := json.Unmarshal(recUpdate.Body.Bytes(), &updatedUser); err != nil {
		t.Fatalf("failed to decode updated user response: %v", err)
	}
	if updatedUser.StatusEmoji == nil || *updatedUser.StatusEmoji != "rocket" {
		t.Fatalf("expected status_emoji='rocket', got %v", updatedUser.StatusEmoji)
	}
	if updatedUser.StatusMessage == nil || *updatedUser.StatusMessage != "Working on AI features" {
		t.Fatalf("expected status_message='Working on AI features', got %v", updatedUser.StatusMessage)
	}

	// 2. Query user status endpoint
	reqGetStatus := httptest.NewRequest(http.MethodGet, "/api/v1/users/user/status", nil)
	reqGetStatus.Header.Set("Authorization", "Bearer "+token)
	recGetStatus := httptest.NewRecorder()
	handler.handleOpenWebUIUsers(recGetStatus, reqGetStatus)

	if recGetStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 on get status, got %d: %s", recGetStatus.Code, recGetStatus.Body.String())
	}
	var statusResp map[string]any
	_ = json.Unmarshal(recGetStatus.Body.Bytes(), &statusResp)
	if statusResp["status_emoji"] != "rocket" || statusResp["status_message"] != "Working on AI features" {
		t.Fatalf("unexpected status response: %v", statusResp)
	}

	// 3. Verify getSessionUser (/api/v1/auths/) returns the status
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/auths/", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+token)
	recAuth := httptest.NewRecorder()
	handler.handleOpenWebUIAuth(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 on /auths/, got %d: %s", recAuth.Code, recAuth.Body.String())
	}
	var sessionUser OpenWebUISessionUserInfoResponse
	_ = json.Unmarshal(recAuth.Body.Bytes(), &sessionUser)
	if sessionUser.StatusEmoji == nil || *sessionUser.StatusEmoji != "rocket" {
		t.Fatalf("expected sessionUser status_emoji='rocket', got %v", sessionUser.StatusEmoji)
	}

	// 4. Clear status
	clearPayload := map[string]any{
		"status_emoji":   "",
		"status_message": "",
	}
	clearBody, _ := json.Marshal(clearPayload)
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/users/user/status/update", bytes.NewReader(clearBody))
	reqClear.Header.Set("Authorization", "Bearer "+token)
	recClear := httptest.NewRecorder()
	handler.handleOpenWebUIUsers(recClear, reqClear)

	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 on clear status, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var clearedUser OpenWebUISessionUserInfoResponse
	_ = json.Unmarshal(recClear.Body.Bytes(), &clearedUser)
	if clearedUser.StatusEmoji != nil || clearedUser.StatusMessage != nil {
		t.Fatalf("expected nil status after clearing, got emoji=%v msg=%v", clearedUser.StatusEmoji, clearedUser.StatusMessage)
	}
}

func TestOpenWebUIModelProfileImage(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Create a model with base64 profile_image_url
	samplePNG := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAArrG4AAAAAElFTkSuQmCC"
	createPayload := map[string]any{
		"id":            "custom-avatar-model",
		"name":          "Avatar AI",
		"base_model_id": "gpt-4o",
		"params":        map[string]any{},
		"meta": map[string]any{
			"profile_image_url": samplePNG,
		},
		"access_grants": []any{},
		"is_active":     true,
	}
	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/models/create", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token-1")
	rec := httptest.NewRecorder()
	handler.handleOpenWebUIModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on create model, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Fetch profile image: GET /api/v1/models/model/profile/image?id=custom-avatar-model
	reqImg := httptest.NewRequest(http.MethodGet, "/api/v1/models/model/profile/image?id=custom-avatar-model", nil)
	reqImg.Header.Set("Authorization", "Bearer test-token-1")
	recImg := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recImg, reqImg)

	if recImg.Code != http.StatusOK {
		t.Fatalf("expected 200 on get profile image, got %d", recImg.Code)
	}
	contentType := recImg.Header().Get("Content-Type")
	if contentType != "image/png" {
		t.Fatalf("expected Content-Type: image/png, got %s", contentType)
	}
	if recImg.Body.Len() == 0 {
		t.Fatalf("expected non-empty image body")
	}

	// 3. Fallback for non-existent model: should redirect (302) to /static/favicon.png
	reqFallback := httptest.NewRequest(http.MethodGet, "/api/v1/models/model/profile/image?id=nonexistent", nil)
	reqFallback.Header.Set("Authorization", "Bearer test-token-1")
	recFallback := httptest.NewRecorder()
	handler.handleOpenWebUIModels(recFallback, reqFallback)

	if recFallback.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for fallback, got %d", recFallback.Code)
	}
	if loc := recFallback.Header().Get("Location"); loc != "/static/favicon.png" {
		t.Fatalf("expected Location /static/favicon.png, got %s", loc)
	}
}

func TestOpenWebUIPagination(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Create a knowledge base
	createPayload := map[string]any{
		"name":        "Test Knowledge",
		"description": "Pagination test",
	}
	body, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/create", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token-1")
	rec := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to create knowledge base: %s", rec.Body.String())
	}

	// 2. Query page 1 of search: should return 1 item and total=1
	reqPage1 := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search?page=1", nil)
	reqPage1.Header.Set("Authorization", "Bearer test-token-1")
	recPage1 := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recPage1, reqPage1)

	if recPage1.Code != http.StatusOK {
		t.Fatalf("expected 200 on page 1, got %d", recPage1.Code)
	}
	var respPage1 OpenWebUIPaginatedListResponse
	if err := json.Unmarshal(recPage1.Body.Bytes(), &respPage1); err != nil {
		t.Fatalf("failed to unmarshal page 1: %v", err)
	}
	if len(respPage1.Items) != 1 || respPage1.Total != 1 {
		t.Fatalf("expected 1 item on page 1, got len=%d total=%d", len(respPage1.Items), respPage1.Total)
	}

	// 3. Query page 2 of search: should return 0 items and total=1 (crucial for OpenWebUI infinite scroll!)
	reqPage2 := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search?page=2", nil)
	reqPage2.Header.Set("Authorization", "Bearer test-token-1")
	recPage2 := httptest.NewRecorder()
	handler.handleOpenWebUIKnowledge(recPage2, reqPage2)

	if recPage2.Code != http.StatusOK {
		t.Fatalf("expected 200 on page 2, got %d", recPage2.Code)
	}
	var respPage2 OpenWebUIPaginatedListResponse
	if err := json.Unmarshal(recPage2.Body.Bytes(), &respPage2); err != nil {
		t.Fatalf("failed to unmarshal page 2: %v", err)
	}
	if len(respPage2.Items) != 0 || respPage2.Total != 1 {
		t.Fatalf("expected 0 items on page 2, got len=%d total=%d", len(respPage2.Items), respPage2.Total)
	}
}

func TestOpenWebUIMCPToolServers(t *testing.T) {
	// Mock MCP Server mimicking mcp-bridge-go / mcp-bridge-cf
	mockMCPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: endpoint\ndata: /mcp?room=testroom\n\n"))
			return
		}
		if r.Method == http.MethodPost {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			method, _ := req["method"].(string)
			id := req["id"]

			w.Header().Set("Content-Type", "application/json")
			switch method {
			case "initialize":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]any{
						"protocolVersion": "2024-11-05",
						"capabilities":    map[string]any{"tools": map[string]any{}},
						"serverInfo":      map[string]any{"name": "mock-mcp-bridge", "version": "1.0"},
					},
				})
			case "tools/list":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]any{
						"tools": []map[string]any{
							{
								"name":        "fetch_page",
								"description": "Fetch web page content",
								"inputSchema": map[string]any{"type": "object"},
							},
						},
					},
				})
			case "tools/call":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": "Page content successfully retrieved"},
						},
						"isError": false,
					},
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result":  map[string]any{},
				})
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer mockMCPServer.Close()

	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Verify MCP tool server endpoint via POST /api/v1/configs/tool_servers/verify
	verifyPayload := map[string]any{
		"url":       mockMCPServer.URL + "/mcp?room=testroom",
		"type":      "mcp",
		"auth_type": "none",
	}
	body, _ := json.Marshal(verifyPayload)
	reqVerify := httptest.NewRequest(http.MethodPost, "/api/v1/configs/tool_servers/verify", bytes.NewReader(body))
	reqVerify.Header.Set("Authorization", "Bearer test-token-1")
	recVerify := httptest.NewRecorder()
	handler.handleOpenWebUIConfigs(recVerify, reqVerify)

	if recVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 on verify MCP tool server, got %d: %s", recVerify.Code, recVerify.Body.String())
	}
	var verifyResp map[string]any
	_ = json.Unmarshal(recVerify.Body.Bytes(), &verifyResp)
	if verifyResp["status"] != true {
		t.Fatalf("expected status=true on verify, got %v", verifyResp)
	}

	// 2. Save tool server via POST /api/v1/configs/tool_servers
	savePayload := map[string]any{
		"TOOL_SERVER_CONNECTIONS": []map[string]any{
			{
				"id":        "bridge-1",
				"name":      "Browser Bridge Go",
				"type":      "mcp",
				"url":       mockMCPServer.URL + "/mcp?room=testroom",
				"auth_type": "none",
				"config":    map[string]any{"enable": true},
				"info":      map[string]any{"id": "bridge-1", "name": "Browser Bridge Go", "description": "MCP Bridge"},
			},
		},
	}
	saveBody, _ := json.Marshal(savePayload)
	reqSave := httptest.NewRequest(http.MethodPost, "/api/v1/configs/tool_servers", bytes.NewReader(saveBody))
	reqSave.Header.Set("Authorization", "Bearer test-token-1")
	recSave := httptest.NewRecorder()
	handler.handleOpenWebUIConfigs(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("expected 200 on save tool server, got %d: %s", recSave.Code, recSave.Body.String())
	}

	// 3. Get tool servers via GET /api/v1/configs/tool_servers
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/configs/tool_servers", nil)
	reqGet.Header.Set("Authorization", "Bearer test-token-1")
	recGet := httptest.NewRecorder()
	handler.handleOpenWebUIConfigs(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get tool servers, got %d: %s", recGet.Code, recGet.Body.String())
	}
	var getResp struct {
		Connections []map[string]any `json:"TOOL_SERVER_CONNECTIONS"`
	}
	_ = json.Unmarshal(recGet.Body.Bytes(), &getResp)
	if len(getResp.Connections) != 1 || getResp.Connections[0]["name"] != "Browser Bridge Go" {
		t.Fatalf("expected 1 saved connection, got %v", getResp.Connections)
	}

	// 4. List tools via GET /api/v1/tools/
	reqTools := httptest.NewRequest(http.MethodGet, "/api/v1/tools/", nil)
	reqTools.Header.Set("Authorization", "Bearer test-token-1")
	recTools := httptest.NewRecorder()
	handler.handleOpenWebUITools(recTools, reqTools)

	if recTools.Code != http.StatusOK {
		t.Fatalf("expected 200 on get tools, got %d: %s", recTools.Code, recTools.Body.String())
	}
	var toolsResp []map[string]any
	_ = json.Unmarshal(recTools.Body.Bytes(), &toolsResp)
	if len(toolsResp) != 1 || toolsResp[0]["name"] != "Browser Bridge Go" {
		t.Fatalf("expected 1 tool returned, got %v", toolsResp)
	}

	// 5. Call tool via POST /api/v1/tools/id/server:mcp:bridge-1/call
	callPayload := map[string]any{
		"name":   "fetch_page",
		"params": map[string]any{"url": "https://example.com"},
	}
	callBody, _ := json.Marshal(callPayload)
	reqCall := httptest.NewRequest(http.MethodPost, "/api/v1/tools/id/server:mcp:bridge-1/call", bytes.NewReader(callBody))
	reqCall.Header.Set("Authorization", "Bearer test-token-1")
	recCall := httptest.NewRecorder()
	handler.handleOpenWebUITools(recCall, reqCall)

	if recCall.Code != http.StatusOK {
		t.Fatalf("expected 200 on call tool, got %d: %s", recCall.Code, recCall.Body.String())
	}
	var callResp map[string]any
	_ = json.Unmarshal(recCall.Body.Bytes(), &callResp)
	if callResp["content"] == nil {
		t.Fatalf("expected tool content result, got %v", callResp)
	}
}

