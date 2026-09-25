package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIRouterModelContexts_CRUDAndAutoDetect(t *testing.T) {
	handler, database, cleanup := setupTestHandler(t)
	defer cleanup()

	subdomain := "testsub"

	// 1. Initially empty
	contexts, err := database.GetAIRouterModelContexts(subdomain)
	if err != nil {
		t.Fatalf("unexpected error getting contexts: %v", err)
	}
	if len(contexts) != 0 {
		t.Fatalf("expected 0 contexts, got %d", len(contexts))
	}

	// 2. Test auto-detect default context size for various models
	if ctx := getDefaultContextSize("gemini-1.5-pro"); ctx != 2000000 {
		t.Errorf("expected 2000000 for gemini-1.5-pro, got %d", ctx)
	}
	if ctx := getDefaultContextSize("claude-3-5-sonnet-20241022"); ctx != 200000 {
		t.Errorf("expected 200000 for claude-3-5-sonnet, got %d", ctx)
	}
	if ctx := getDefaultContextSize("gpt-4o-mini"); ctx != 128000 {
		t.Errorf("expected 128000 for gpt-4o-mini, got %d", ctx)
	}
	if ctx := getDefaultContextSize("qwen-2.5-coder-32b"); ctx != 128000 {
		t.Errorf("expected 128000 for qwen-2.5-coder, got %d", ctx)
	}
	if ctx := getDefaultContextSize("gemma-2-9b-it"); ctx != 8192 {
		t.Errorf("expected 8192 for gemma-2, got %d", ctx)
	}

	// 3. Upsert a custom override
	customModel := "custom-local-llama"
	err = database.UpsertAIRouterModelContext(subdomain, customModel, 65536)
	if err != nil {
		t.Fatalf("failed to upsert model context: %v", err)
	}

	mc, err := database.GetAIRouterModelContext(subdomain, customModel)
	if err != nil || mc == nil {
		t.Fatalf("failed to get upserted model context: %v", err)
	}
	if mc.ContextSize != 65536 {
		t.Errorf("expected 65536, got %d", mc.ContextSize)
	}

	// 4. Test getModelContextSize resolves custom override
	resolvedCtx := handler.getModelContextSize(subdomain, customModel)
	if resolvedCtx != 65536 {
		t.Errorf("expected custom size 65536, got %d", resolvedCtx)
	}

	// 5. Test auto-detect API endpoint
	autoDetectReqBody := map[string]interface{}{
		"models": []string{"gpt-4o", "claude-3-5-sonnet", customModel},
		"save":   true,
	}
	bodyBytes, _ := json.Marshal(autoDetectReqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/airouter/contexts/auto-detect", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	handler.handleAutoDetectModelContexts(rec, req, subdomain)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status   string `json:"status"`
		Detected []struct {
			ModelName   string `json:"model_name"`
			ContextSize int    `json:"context_size"`
			IsCustom    bool   `json:"is_custom"`
		} `json:"detected"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Detected) != 3 {
		t.Fatalf("expected 3 detected models, got %d", len(resp.Detected))
	}

	// 6. Delete context
	err = database.DeleteAIRouterModelContext(subdomain, mc.ID)
	if err != nil {
		t.Fatalf("failed to delete model context: %v", err)
	}
}
