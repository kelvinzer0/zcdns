package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

