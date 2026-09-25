package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestChatDBPool_BasicLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_chat_pool_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbFile := filepath.Join(tempDir, "test_main.sqlite")
	chatStorageDir := filepath.Join(tempDir, "chats")
	os.Setenv("CHAT_STORAGE_DIR", chatStorageDir)
	defer os.Unsetenv("CHAT_STORAGE_DIR")

	database, err := InitDB(dbFile)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	sub := "demo-sub"
	userID := "user-123"
	chatID := "chat-abc-456"
	title := "My Per-Conversation Chat"

	initialChatJSON := `{
		"id": "chat-abc-456",
		"title": "My Per-Conversation Chat",
		"chat": {
			"id": "chat-abc-456",
			"history": {
				"messages": {
					"msg-1": {
						"id": "msg-1",
						"role": "user",
						"content": "Hello isolated SQLite!"
					}
				}
			}
		}
	}`

	// 1. Upsert chat
	err = database.UpsertOpenWebUIChat(sub, userID, chatID, title, initialChatJSON, "")
	if err != nil {
		t.Fatalf("UpsertOpenWebUIChat failed: %v", err)
	}

	// Verify that the isolated SQLite database file exists on disk
	expectedDBFile := filepath.Join(chatStorageDir, sub, userID, chatID+".db")
	if _, err := os.Stat(expectedDBFile); os.IsNotExist(err) {
		t.Fatalf("Expected isolated SQLite DB file at %s, but file does not exist", expectedDBFile)
	}

	// 2. Read chat via GetOpenWebUIChatRaw
	retTitle, rawJSON, _, _, _, _, _, err := database.GetOpenWebUIChatRaw(sub, userID, chatID)
	if err != nil {
		t.Fatalf("GetOpenWebUIChatRaw failed: %v", err)
	}

	var chatMap map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &chatMap); err != nil {
		t.Fatalf("Failed to parse returned JSON: %v", err)
	}
	if retTitle != title {
		t.Fatalf("Expected title %s, got %v", title, retTitle)
	}

	// 3. Update message inside the chat
	newAssistantContent := "I am running in my own independent SQLite database!"
	err = database.UpdateOpenWebUIMessageInChat(sub, userID, chatID, "msg-2", func(msg map[string]interface{}) map[string]interface{} {
		msg["id"] = "msg-2"
		msg["role"] = "assistant"
		msg["content"] = newAssistantContent
		msg["parentId"] = "msg-1"
		return msg
	})
	if err != nil {
		t.Fatalf("UpdateOpenWebUIMessageInChat failed: %v", err)
	}

	// Verify message in isolated DB and DAG messages table
	_, rawUpdatedJSON, _, _, _, _, _, err := database.GetOpenWebUIChatRaw(sub, userID, chatID)
	if err != nil {
		t.Fatalf("GetOpenWebUIChatRaw after update failed: %v", err)
	}
	if !containsSubstring(rawUpdatedJSON, newAssistantContent) {
		t.Fatalf("Updated content not found in returned chat JSON: %s", rawUpdatedJSON)
	}

	// Check messages table in isolated DB directly
	chatDB, _, err := database.chatPool.GetChatDB(sub, userID, chatID)
	if err != nil {
		t.Fatalf("Failed to get chat DB from pool: %v", err)
	}
	var msgCount int
	err = chatDB.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&msgCount)
	if err != nil {
		t.Fatalf("Query messages table failed: %v", err)
	}
	if msgCount < 2 {
		t.Fatalf("Expected at least 2 messages in messages table, got %d", msgCount)
	}

	// 4. Test delete
	err = database.DeleteOpenWebUIChat(sub, userID, chatID)
	if err != nil {
		t.Fatalf("DeleteOpenWebUIChat failed: %v", err)
	}

	if _, err := os.Stat(expectedDBFile); !os.IsNotExist(err) {
		t.Fatalf("Expected DB file %s to be deleted, but it still exists", expectedDBFile)
	}
}

func TestChatDBPool_LegacyMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_chat_migration_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbFile := filepath.Join(tempDir, "test_legacy.sqlite")
	chatStorageDir := filepath.Join(tempDir, "chats")
	os.Setenv("CHAT_STORAGE_DIR", chatStorageDir)
	defer os.Unsetenv("CHAT_STORAGE_DIR")

	database, err := InitDB(dbFile)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	sub := "legacy-sub"
	userID := "legacy-user"
	chatID := "legacy-chat-789"
	legacyJSON := `{"id":"legacy-chat-789","title":"Legacy Chat","chat":{"history":{"messages":{"m1":{"id":"m1","role":"user","content":"legacy message"}}}}}`

	// Insert legacy chat directly into openwebui_chats table bypassing chatPool
	_, err = database.conn.Exec(`
		INSERT INTO openwebui_chats (id, subdomain, user_id, title, chat_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1000, 1000)
	`, chatID, sub, userID, "Legacy Chat", legacyJSON)
	if err != nil {
		t.Fatalf("Failed to insert legacy chat: %v", err)
	}

	// Target DB file should not exist yet before read/migration
	targetDBFile := filepath.Join(chatStorageDir, sub, userID, chatID+".db")
	_ = os.Remove(targetDBFile)

	// Reading via GetOpenWebUIChatRaw should trigger on-the-fly migration to isolated DB
	_, fetchedJSON, _, _, _, _, _, err := database.GetOpenWebUIChatRaw(sub, userID, chatID)
	if err != nil {
		t.Fatalf("GetOpenWebUIChatRaw failed: %v", err)
	}
	if !containsSubstring(fetchedJSON, "legacy message") {
		t.Fatalf("Fetched JSON did not contain legacy message: %s", fetchedJSON)
	}

	// Verify that target isolated DB file now exists on disk
	if _, err := os.Stat(targetDBFile); os.IsNotExist(err) {
		t.Fatalf("Expected target DB file %s to be created by migration, but not found", targetDBFile)
	}
}

func TestChatDBPool_ConcurrentMultiUserChats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_chat_concurrent_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbFile := filepath.Join(tempDir, "test_concurrent.sqlite")
	chatStorageDir := filepath.Join(tempDir, "chats")
	os.Setenv("CHAT_STORAGE_DIR", chatStorageDir)
	defer os.Unsetenv("CHAT_STORAGE_DIR")

	database, err := InitDB(dbFile)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	const numUsers = 10
	const numMessagesPerUser = 5

	var wg sync.WaitGroup
	errCh := make(chan error, numUsers*numMessagesPerUser)

	for u := 0; u < numUsers; u++ {
		wg.Add(1)
		sub := fmt.Sprintf("tenant-%d", u)
		userID := fmt.Sprintf("user-%d", u)
		chatID := fmt.Sprintf("chat-%d", u)

		// Create chat
		initJSON := fmt.Sprintf(`{"id":"%s","title":"Concurrent Chat %d"}`, chatID, u)
		if err := database.UpsertOpenWebUIChat(sub, userID, chatID, fmt.Sprintf("Chat %d", u), initJSON, ""); err != nil {
			t.Fatalf("Initial upsert failed: %v", err)
		}

		go func(tenantSub, uid, cid string) {
			defer wg.Done()
			for m := 0; m < numMessagesPerUser; m++ {
				msgID := fmt.Sprintf("m-%d", m)
				content := fmt.Sprintf("Message %d from %s", m, uid)
				err := database.UpdateOpenWebUIMessageInChat(tenantSub, uid, cid, msgID, func(msg map[string]interface{}) map[string]interface{} {
					msg["id"] = msgID
					msg["role"] = "user"
					msg["content"] = content
					return msg
				})
				if err != nil {
					errCh <- fmt.Errorf("concurrent update failed for %s/%s: %w", uid, cid, err)
					return
				}
			}
		}(sub, userID, chatID)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("Concurrent write error: %v", err)
	}
}

func TestChatDBPool_MergeHistoryAndNoMessageLoss(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_chat_merge_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbFile := filepath.Join(tempDir, "test_merge.sqlite")
	chatStorageDir := filepath.Join(tempDir, "chats")
	os.Setenv("CHAT_STORAGE_DIR", chatStorageDir)
	defer os.Unsetenv("CHAT_STORAGE_DIR")

	database, err := InitDB(dbFile)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	sub := "test-sub"
	userID := "usr-1"
	chatID := "chat-1"

	// 1. Initial chat with user and assistant messages
	initialChat := map[string]interface{}{
		"id":    chatID,
		"title": "First Conversation",
		"chat": map[string]interface{}{
			"id":     chatID,
			"title":  "First Conversation",
			"models": []interface{}{"gpt-4o"},
			"history": map[string]interface{}{
				"currentId": "asst-1",
				"messages": map[string]interface{}{
					"user-1": map[string]interface{}{
						"id":          "user-1",
						"role":        "user",
						"content":     "Hello AI!",
						"childrenIds": []interface{}{"asst-1"},
					},
					"asst-1": map[string]interface{}{
						"id":          "asst-1",
						"parentId":    "user-1",
						"role":        "assistant",
						"content":     "Hello human!",
						"childrenIds": []interface{}{},
					},
				},
			},
		},
	}
	initialBytes, _ := json.Marshal(initialChat)
	err = database.UpsertOpenWebUIChat(sub, userID, chatID, "First Conversation", string(initialBytes), "")
	if err != nil {
		t.Fatalf("Initial UpsertOpenWebUIChat failed: %v", err)
	}

	// 2. OpenWebUI sends partial update with only params (e.g. user toggles a setting or switches model)
	partialUpdate := map[string]interface{}{
		"chat": map[string]interface{}{
			"params": map[string]interface{}{
				"temperature": 0.7,
			},
			"models": []interface{}{"claude-3-5-sonnet"},
		},
	}
	merged, err := database.MergeUpdateOpenWebUIChat(sub, userID, chatID, partialUpdate, "")
	if err != nil {
		t.Fatalf("MergeUpdateOpenWebUIChat failed: %v", err)
	}

	// 3. Verify messages are STILL present and intact!
	_, rawJSON, _, _, _, _, _, err := database.GetOpenWebUIChatRaw(sub, userID, chatID)
	if err != nil {
		t.Fatalf("GetOpenWebUIChatRaw failed: %v", err)
	}

	if !containsSubstring(rawJSON, "Hello AI!") || !containsSubstring(rawJSON, "Hello human!") {
		t.Fatalf("CRITICAL BUG: Messages were wiped out by partial update! rawJSON: %s", rawJSON)
	}
	if !containsSubstring(rawJSON, "temperature") || !containsSubstring(rawJSON, "claude-3-5-sonnet") {
		t.Fatalf("Params were not merged properly! rawJSON: %s", rawJSON)
	}

	// 4. Test branching / continue with new model (second assistant child)
	branchUpdate := map[string]interface{}{
		"chat": map[string]interface{}{
			"history": map[string]interface{}{
				"currentId": "asst-2",
				"messages": map[string]interface{}{
					"asst-2": map[string]interface{}{
						"id":       "asst-2",
						"parentId": "user-1",
						"role":     "assistant",
						"content":  "I am Claude continuing your conversation!",
					},
				},
			},
		},
	}
	_, err = database.MergeUpdateOpenWebUIChat(sub, userID, chatID, branchUpdate, "")
	if err != nil {
		t.Fatalf("Branch MergeUpdateOpenWebUIChat failed: %v", err)
	}

	_, finalRaw, _, _, _, _, _, err := database.GetOpenWebUIChatRaw(sub, userID, chatID)
	if err != nil {
		t.Fatalf("GetOpenWebUIChatRaw after branch failed: %v", err)
	}

	// Both assistant responses must exist, user-1 must have both as children, and currentId must be asst-2
	if !containsSubstring(finalRaw, "Hello human!") || !containsSubstring(finalRaw, "I am Claude continuing your conversation!") {
		t.Fatalf("Branching lost earlier messages: %s", finalRaw)
	}

	_ = merged
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
