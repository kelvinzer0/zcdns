package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestOpenWebUIUserUsage(t *testing.T) {
	handler, database, cleanup := setupTestHandler(t)
	defer cleanup()

	subdomain := "usage-sub"
	userID := "usr_usage123"

	// 1. Test GET /api/v1/users/usage with no chats
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/usage", nil)
	req.Header.Set("X-Subdomain", subdomain)
	req.Header.Set("Authorization", "Bearer test-token-"+userID)

	rec := httptest.NewRecorder()
	handler.handleOpenWebUIUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var usageResp OpenWebUIUserUsageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &usageResp); err != nil {
		t.Fatalf("failed to unmarshal usage response: %v", err)
	}

	if usageResp.Totals.Messages != 0 {
		t.Errorf("expected 0 messages, got %d", usageResp.Totals.Messages)
	}
	if usageResp.Totals.LifetimeTokens != 0 {
		t.Errorf("expected 0 lifetime tokens, got %d", usageResp.Totals.LifetimeTokens)
	}
	if len(usageResp.Heatmap) == 0 {
		t.Errorf("expected non-empty heatmap array covering period, got 0 entries")
	}
	if len(usageResp.WeeklyHeatmap) == 0 {
		t.Errorf("expected non-empty weekly heatmap, got 0 entries")
	}
	if len(usageResp.CumulativeHeatmap) == 0 {
		t.Errorf("expected non-empty cumulative heatmap, got 0 entries")
	}

	// 2. Insert test chats with messages and token usages
	now := time.Now().Unix()
	chatJSON := `{
		"chat": {
			"history": {
				"messages": {
					"m1": {
						"id": "m1",
						"role": "user",
						"content": "Hello AI!",
						"timestamp": ` + strconv.FormatInt(now-100, 10) + `,
						"info": {
							"usage": {
								"prompt_tokens": 15,
								"completion_tokens": 0,
								"total_tokens": 15
							}
						}
					},
					"m2": {
						"id": "m2",
						"role": "assistant",
						"model": "gpt-4o-mini",
						"content": "Hello! How can I help you today?",
						"timestamp": ` + strconv.FormatInt(now-90, 10) + `,
						"info": {
							"usage": {
								"prompt_tokens": 0,
								"completion_tokens": 25,
								"total_tokens": 25
							}
						}
					}
				}
			}
		}
	}`

	resolvedUser := handler.resolveUserFromToken(subdomain, "test-token-"+userID)
	if err := database.UpsertOpenWebUIChat(subdomain, resolvedUser.ID, "chat-usage-1", "Usage Chat 1", chatJSON, ""); err != nil {
		t.Fatalf("UpsertOpenWebUIChat failed: %v", err)
	}

	// 3. Test GET /api/v1/users/usage with chats present
	reqWithChat := httptest.NewRequest(http.MethodGet, "/api/v1/users/usage", nil)
	reqWithChat.Header.Set("X-Subdomain", subdomain)
	reqWithChat.Header.Set("Authorization", "Bearer test-token-"+userID)

	recWithChat := httptest.NewRecorder()
	handler.handleOpenWebUIUsers(recWithChat, reqWithChat)

	if recWithChat.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recWithChat.Code, recWithChat.Body.String())
	}

	var usageWithChat OpenWebUIUserUsageResponse
	if err := json.Unmarshal(recWithChat.Body.Bytes(), &usageWithChat); err != nil {
		t.Fatalf("failed to unmarshal usage response: %v", err)
	}

	if usageWithChat.Totals.TotalChats != 1 {
		t.Errorf("expected 1 total chat, got %d", usageWithChat.Totals.TotalChats)
	}
	if usageWithChat.Totals.Messages != 2 {
		t.Errorf("expected 2 messages, got %d", usageWithChat.Totals.Messages)
	}
	if usageWithChat.Totals.UserMessages != 1 {
		t.Errorf("expected 1 user message, got %d", usageWithChat.Totals.UserMessages)
	}
	if usageWithChat.Totals.AssistantMessages != 1 {
		t.Errorf("expected 1 assistant message, got %d", usageWithChat.Totals.AssistantMessages)
	}
	if usageWithChat.Totals.LifetimeTokens != 40 {
		t.Errorf("expected 40 lifetime tokens, got %d", usageWithChat.Totals.LifetimeTokens)
	}
	if len(usageWithChat.TopModels) != 1 || usageWithChat.TopModels[0].ModelID != "gpt-4o-mini" {
		t.Errorf("expected top model gpt-4o-mini, got %+v", usageWithChat.TopModels)
	}
	if usageWithChat.Insights.MostUsedModel == nil || *usageWithChat.Insights.MostUsedModel != "gpt-4o-mini" {
		t.Errorf("expected most used model gpt-4o-mini, got %v", usageWithChat.Insights.MostUsedModel)
	}
	if usageWithChat.Totals.CurrentStreak < 1 {
		t.Errorf("expected current streak >= 1, got %d", usageWithChat.Totals.CurrentStreak)
	}

	// 4. Test alias GET /api/v1/users/user/usage
	reqAlias := httptest.NewRequest(http.MethodGet, "/api/v1/users/user/usage", nil)
	reqAlias.Header.Set("X-Subdomain", subdomain)
	reqAlias.Header.Set("Authorization", "Bearer test-token-"+userID)

	recAlias := httptest.NewRecorder()
	handler.handleOpenWebUIUsers(recAlias, reqAlias)

	if recAlias.Code != http.StatusOK {
		t.Fatalf("expected alias 200, got %d: %s", recAlias.Code, recAlias.Body.String())
	}

	// 5. Test auxiliary user routes
	testAux := func(path string) {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Subdomain", subdomain)
		r.Header.Set("Authorization", "Bearer test-token-"+userID)
		w := httptest.NewRecorder()
		handler.handleOpenWebUIUsers(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d: %s", path, w.Code, w.Body.String())
		}
	}

	testAux("/api/v1/users/user/info")
	testAux("/api/v1/users/user/variables")
	testAux("/api/v1/users/groups")
	testAux("/api/v1/users/default/permissions")
	testAux("/api/v1/users/default/permissions/defaults")
}
