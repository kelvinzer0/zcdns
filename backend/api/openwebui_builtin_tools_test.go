package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zcdns-backend/db"
)

func TestBuiltinTools_SpecificationAudit(t *testing.T) {
	specs := GetBuiltinToolSpecs()
	if len(specs) == 0 {
		t.Fatalf("expected non-empty builtin tool specs")
	}

	toolNames := make(map[string]bool)
	for _, s := range specs {
		fnName, _ := s.Function["name"].(string)
		toolNames[fnName] = true
	}

	// Verify required active tools are present
	requiredTools := []string{
		"get_current_timestamp",
		"calculate_timestamp",
		"ask_user",
		"search_chats",
		"view_chat",
		"list_memories",
		"search_memories",
		"add_memory",
		"update_memory",
		"delete_memory",
		"list_chat_files",
		"view_file",
		"query_chat_files",
		"search_knowledge_bases",
		"query_knowledge_files",
		"view_knowledge_file",
		"generate_image",
		"edit_image",
	}
	for _, req := range requiredTools {
		if !toolNames[req] {
			t.Errorf("missing required builtin tool: %s", req)
		}
	}

	// Verify that unsupported tasks/automations/calendar tools are strictly EXCLUDED as requested
	excludedTools := []string{
		"create_tasks",
		"update_task",
		"create_automation",
		"update_automation",
		"list_automations",
		"search_calendar_events",
		"create_calendar_event",
	}
	for _, exc := range excludedTools {
		if toolNames[exc] {
			t.Errorf("tool %s should be excluded as it is not yet supported", exc)
		}
	}
}

func TestBuiltinTools_Execution(t *testing.T) {
	handler, database, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	subdomain := "tenant-test"
	userID := "usr-123"
	chatID := "chat-xyz"

	// 1. Test get_current_timestamp
	tsRes, err := handler.ExecuteBuiltinTool(ctx, "get_current_timestamp", map[string]any{}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("get_current_timestamp failed: %v", err)
	}
	tsMap, ok := tsRes.(map[string]any)
	if !ok || tsMap["current_timestamp"] == nil || tsMap["date_human"] == nil {
		t.Fatalf("invalid get_current_timestamp result: %v", tsRes)
	}

	// 2. Test calculate_timestamp
	calcRes, err := handler.ExecuteBuiltinTool(ctx, "calculate_timestamp", map[string]any{
		"days_ago": float64(3),
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("calculate_timestamp failed: %v", err)
	}
	calcMap, ok := calcRes.(map[string]any)
	if !ok || calcMap["calculated_timestamp"] == nil {
		t.Fatalf("invalid calculate_timestamp result: %v", calcRes)
	}

	// 3. Test ask_user
	askRes, err := handler.ExecuteBuiltinTool(ctx, "ask_user", map[string]any{
		"questions": []any{
			map[string]any{
				"id":       "q1",
				"question": "Which language do you prefer?",
				"options": []any{
					map[string]any{"label": "Go", "description": "High performance"},
					map[string]any{"label": "Python", "description": "Easy scripting"},
				},
			},
		},
		"allow_other": true,
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("ask_user failed: %v", err)
	}
	askMap, ok := askRes.(map[string]any)
	if !ok || askMap["status"] != "prompt_required" {
		t.Fatalf("invalid ask_user result: %v", askRes)
	}

	// 4. Test Memories Tools
	addRes, err := handler.ExecuteBuiltinTool(ctx, "add_memory", map[string]any{
		"content": "User prefers dark mode and concise code explanations.",
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("add_memory failed: %v", err)
	}
	addMap, _ := addRes.(map[string]any)
	memID, _ := addMap["id"].(string)
	if memID == "" {
		t.Fatalf("expected memory id returned: %v", addRes)
	}

	listRes, err := handler.ExecuteBuiltinTool(ctx, "list_memories", map[string]any{}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("list_memories failed: %v", err)
	}
	memList, _ := listRes.([]db.OpenWebUIMemoryDB)
	if len(memList) != 1 || memList[0].Content != "User prefers dark mode and concise code explanations." {
		t.Fatalf("unexpected list_memories result: %v", listRes)
	}

	searchRes, err := handler.ExecuteBuiltinTool(ctx, "search_memories", map[string]any{
		"query": "dark mode",
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("search_memories failed: %v", err)
	}
	sList, _ := searchRes.([]db.OpenWebUIMemoryDB)
	if len(sList) != 1 {
		t.Fatalf("expected 1 memory match, got %d", len(sList))
	}

	_, err = handler.ExecuteBuiltinTool(ctx, "update_memory", map[string]any{
		"id":      memID,
		"content": "User prefers high contrast dark mode.",
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("update_memory failed: %v", err)
	}

	_, err = handler.ExecuteBuiltinTool(ctx, "delete_memory", map[string]any{
		"id": memID,
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("delete_memory failed: %v", err)
	}

	// 5. Test search_chats and view_chat
	chatJSON := `{"chat":{"history":{"messages":{"m1":{"role":"user","content":"Discussing Kubernetes cluster networking"}}}},"timestamp":1700000000}`
	_ = database.UpsertOpenWebUIChat(subdomain, userID, "chat-earlier-1", "K8s Discussion", chatJSON, "")

	chatsRes, err := handler.ExecuteBuiltinTool(ctx, "search_chats", map[string]any{
		"query": "K8s",
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("search_chats failed: %v", err)
	}
	cList, _ := chatsRes.([]map[string]any)
	if len(cList) != 1 || cList[0]["title"] != "K8s Discussion" {
		t.Fatalf("expected 1 chat search result, got %v", chatsRes)
	}

	viewChatRes, err := handler.ExecuteBuiltinTool(ctx, "view_chat", map[string]any{
		"chat_id": "chat-earlier-1",
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("view_chat failed: %v", err)
	}
	vcMap, _ := viewChatRes.(map[string]any)
	if vcMap["title"] != "K8s Discussion" {
		t.Fatalf("view_chat returned unexpected title: %v", vcMap)
	}

	// 6. Test Files Tools
	fRec := db.OpenWebUIFileDB{
		ID:          "file-doc-1",
		Subdomain:   subdomain,
		UserID:      userID,
		Filename:    "spec.txt",
		ContentType: "text/plain",
		Size:        24,
		DataJSON:    "Specification text content",
	}
	_ = database.InsertOpenWebUIFile(fRec)

	filesRes, err := handler.ExecuteBuiltinTool(ctx, "list_chat_files", map[string]any{}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("list_chat_files failed: %v", err)
	}
	flList, _ := filesRes.([]map[string]any)
	if len(flList) != 1 || flList[0]["filename"] != "spec.txt" {
		t.Fatalf("unexpected list_chat_files: %v", filesRes)
	}

	viewFileRes, err := handler.ExecuteBuiltinTool(ctx, "view_file", map[string]any{
		"file_id": "file-doc-1",
	}, subdomain, userID, chatID)
	if err != nil {
		t.Fatalf("view_file failed: %v", err)
	}
	vfMap, _ := viewFileRes.(map[string]any)
	if vfMap["filename"] != "spec.txt" || vfMap["content"] != "Specification text content" {
		t.Fatalf("unexpected view_file: %v", viewFileRes)
	}
}

func TestMemories_HTTPEndpoints(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// 1. Add memory via POST /api/v1/memories/add
	addPayload := []byte(`{"content": "Prefers Indonesian language for explanations", "type": "context"}`)
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/v1/memories/add", bytes.NewReader(addPayload))
	reqAdd.Host = "testsub.router.zcdns.id"
	reqAdd.Header.Set("Authorization", "Bearer usr_test_token")
	recAdd := httptest.NewRecorder()
	handler.handleOpenWebUIMemories(recAdd, reqAdd)

	if recAdd.Code != http.StatusOK {
		t.Fatalf("expected 200 on add memory, got %d: %s", recAdd.Code, recAdd.Body.String())
	}
	var createdMem db.OpenWebUIMemoryDB
	_ = json.Unmarshal(recAdd.Body.Bytes(), &createdMem)
	if createdMem.ID == "" {
		t.Fatalf("expected memory ID created: %v", createdMem)
	}

	// 2. Get memories via GET /api/v1/memories/
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/memories/", nil)
	reqGet.Host = "testsub.router.zcdns.id"
	reqGet.Header.Set("Authorization", "Bearer usr_test_token")
	recGet := httptest.NewRecorder()
	handler.handleOpenWebUIMemories(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get memories, got %d", recGet.Code)
	}
	var memList []db.OpenWebUIMemoryDB
	_ = json.Unmarshal(recGet.Body.Bytes(), &memList)
	if len(memList) != 1 || memList[0].Content != "Prefers Indonesian language for explanations" {
		t.Fatalf("unexpected memList: %v", memList)
	}

	// 3. Search memories via POST /api/v1/memories/search
	searchPayload := []byte(`{"query": "Indonesian"}`)
	reqSearch := httptest.NewRequest(http.MethodPost, "/api/v1/memories/search", bytes.NewReader(searchPayload))
	reqSearch.Host = "testsub.router.zcdns.id"
	reqSearch.Header.Set("Authorization", "Bearer usr_test_token")
	recSearch := httptest.NewRecorder()
	handler.handleOpenWebUIMemories(recSearch, reqSearch)

	if recSearch.Code != http.StatusOK {
		t.Fatalf("expected 200 on search memories, got %d", recSearch.Code)
	}
	var sResult []db.OpenWebUIMemoryDB
	_ = json.Unmarshal(recSearch.Body.Bytes(), &sResult)
	if len(sResult) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(sResult))
	}

	// 4. Delete memory via DELETE /api/v1/memories/:id
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/memories/"+createdMem.ID, nil)
	reqDel.Host = "testsub.router.zcdns.id"
	reqDel.Header.Set("Authorization", "Bearer usr_test_token")
	recDel := httptest.NewRecorder()
	handler.handleOpenWebUIMemories(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete memory, got %d", recDel.Code)
	}
}

func TestImageGeneration_Endpoints(t *testing.T) {
	handler, database, cleanup := setupTestHandler(t)
	defer cleanup()

	// Mock upstream OpenAI server for image generation
	upstreamMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/generations") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"created": 1727200000,
				"data": [
					{"url": "https://images.example.com/generated-img-123.png"}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstreamMock.Close()

	// Configure AI Router connection pointing to mock upstream
	_, err := database.AddAIRouterConnection(&db.AIRouterConnection{
		Subdomain:  "testsub",
		Provider:   "openai",
		APIType:    "openai",
		Name:       "Mock OpenAI",
		APIKey:     "sk-test-openai-key",
		BaseURL:    upstreamMock.URL,
		ModelsJSON: `["dall-e-3"]`,
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("failed to create router connection: %v", err)
	}

	// 1. Test POST /v1/images/generations
	genPayload := []byte(`{"prompt": "A cybernetic futuristic lion in neon city", "size": "1024x1024", "model": "dall-e-3"}`)
	reqGen := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(genPayload))
	reqGen.Host = "testsub.router.zcdns.id"
	recGen := httptest.NewRecorder()
	handler.handleAIRouterImages(recGen, reqGen)

	if recGen.Code != http.StatusOK {
		t.Fatalf("expected 200 on image generation, got %d: %s", recGen.Code, recGen.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(recGen.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse image generation response: %v", err)
	}
	dataArr, ok := res["data"].([]any)
	if !ok || len(dataArr) != 1 {
		t.Fatalf("expected 1 image in response data: %v", res)
	}
	imgObj, _ := dataArr[0].(map[string]any)
	if imgObj["url"] != "https://images.example.com/generated-img-123.png" {
		t.Fatalf("unexpected image url returned: %v", imgObj)
	}
}
