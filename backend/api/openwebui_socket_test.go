package api

import (
	"testing"
	"time"
)

func TestParseSocketIOEvent(t *testing.T) {
	msg := `42["user-join",{"auth":{"token":"xyz"}}]`
	ackID, name, data := parseSocketIOEvent(msg)
	if ackID != "" {
		t.Fatalf("expected empty ackID, got %s", ackID)
	}
	if name != "user-join" {
		t.Fatalf("expected 'user-join', got %s", name)
	}
	auth, ok := data["auth"].(map[string]interface{})
	if !ok || auth["token"] != "xyz" {
		t.Fatalf("expected token 'xyz', got %v", data)
	}

	// With ackID
	msgWithAck := `42101["usage",{"model":"gpt-4o"}]`
	ackID, name, data = parseSocketIOEvent(msgWithAck)
	if ackID != "101" {
		t.Fatalf("expected ackID '101', got %s", ackID)
	}
	if name != "usage" {
		t.Fatalf("expected 'usage', got %s", name)
	}
	if data["model"] != "gpt-4o" {
		t.Fatalf("expected model gpt-4o, got %v", data["model"])
	}
}

func TestParseSocketIOAck(t *testing.T) {
	msg := `4342[{"result":"success"}]`
	ackID, result := parseSocketIOAck(msg)
	if ackID != 42 {
		t.Fatalf("expected ackID 42, got %d", ackID)
	}
	resMap, ok := result.(map[string]interface{})
	if !ok || resMap["result"] != "success" {
		t.Fatalf("expected result success, got %v", result)
	}
}

func TestNormalizeDocumentID(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"doc_12345", "12345"},
		{"/note:67890", "note:67890"},
		{"doc_/abc", "abc"},
		{"plain_doc", "plain_doc"},
	}
	for _, c := range cases {
		got := normalizeDocumentID(c.input)
		if got != c.expected {
			t.Errorf("normalizeDocumentID(%s) = %s; want %s", c.input, got, c.expected)
		}
	}
}

func TestSocketHubBasics(t *testing.T) {
	hub := &SocketHub{
		clients:     make(map[string]*SocketClient),
		rooms:       make(map[string]map[string]*SocketClient),
		usagePool:   make(map[string]map[string]int64),
		ydocUpdates: make(map[string][]any),
	}

	client := &SocketClient{
		sid:         "sid_1",
		userID:      "u_100",
		userName:    "Alice",
		lastSeenAt:  time.Now().Unix(),
		rooms:       make(map[string]bool),
		pendingAcks: make(map[int64]chan any),
	}

	hub.register(client)
	if hub.getUserCount() != 1 {
		t.Fatalf("expected user count 1, got %d", hub.getUserCount())
	}
	if hub.getClient("sid_1") != client {
		t.Fatalf("expected to get registered client")
	}

	hub.joinRoom(client, "channel:general")
	sids := hub.getRoomSids("channel:general")
	if len(sids) != 1 || sids[0] != "sid_1" {
		t.Fatalf("expected [sid_1] in room, got %v", sids)
	}

	hub.recordUsage("gpt-4o", "sid_1")
	models := hub.getModelsInUse()
	if len(models) != 1 || models[0] != "gpt-4o" {
		t.Fatalf("expected [gpt-4o] in models in use, got %v", models)
	}

	// YDoc updates
	hub.appendYDocUpdate("note_1", map[string]string{"type": "insert"})
	updates := hub.getYDocState("note_1")
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	hub.clearYDoc("note_1")
	if len(hub.getYDocState("note_1")) != 0 {
		t.Fatalf("expected 0 updates after clear")
	}

	hub.leaveRoom(client, "channel:general")
	if len(hub.getRoomSids("channel:general")) != 0 {
		t.Fatalf("expected 0 sids in room after leave")
	}

	hub.unregister(client)
	if hub.getUserCount() != 0 {
		t.Fatalf("expected user count 0 after unregister, got %d", hub.getUserCount())
	}
}

func TestSocketClientAckResolution(t *testing.T) {
	client := &SocketClient{
		sid:         "sid_test",
		pendingAcks: make(map[int64]chan any),
	}
	respCh := make(chan any, 1)
	client.pendingAcks[1] = respCh

	expectedData := map[string]any{"status": "ok"}
	client.resolveAck(1, expectedData)

	select {
	case res := <-respCh:
		m, ok := res.(map[string]any)
		if !ok || m["status"] != "ok" {
			t.Fatalf("unexpected ack data: %v", res)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout waiting for ack resolution")
	}
}
