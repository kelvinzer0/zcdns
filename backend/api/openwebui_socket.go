package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SocketClient represents an active WebSocket connection
type SocketClient struct {
	conn        *websocket.Conn
	sid         string
	subdomain   string
	userID      string
	userName    string
	role        string
	email       string
	lastSeenAt  int64
	rooms       map[string]bool
	send        chan []byte
	closed      bool
	closeMu     sync.Mutex
	writeMu     sync.Mutex
	ackCounter  int64
	ackMu       sync.Mutex
	pendingAcks map[int64]chan any
}

func (c *SocketClient) close() {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return
	}
	c.closed = true
	c.closeMu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
	}
	if c.send != nil {
		close(c.send)
	}
}

func (c *SocketClient) write(msg []byte) error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return fmt.Errorf("client closed")
	}
	c.closeMu.Unlock()

	if c.send == nil {
		return c.writeDirect(msg)
	}

	select {
	case c.send <- msg:
		return nil
	default:
		// Queue full, write direct with short deadline so frames are delivered
		return c.writeDirect(msg)
	}
}

func (c *SocketClient) writeDirect(msg []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("no connection")
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

func (c *SocketClient) writePump() {
	for msg := range c.send {
		if err := c.writeDirect(msg); err != nil {
			break
		}
	}
}

// Call sends an RPC event to the client and blocks until the client returns an ACK or timeout expires
func (c *SocketClient) Call(event string, data any, timeout time.Duration) (any, error) {
	c.ackMu.Lock()
	c.ackCounter++
	ackID := c.ackCounter
	respCh := make(chan any, 1)
	if c.pendingAcks == nil {
		c.pendingAcks = make(map[int64]chan any)
	}
	c.pendingAcks[ackID] = respCh
	c.ackMu.Unlock()

	defer func() {
		c.ackMu.Lock()
		delete(c.pendingAcks, ackID)
		c.ackMu.Unlock()
	}()

	payloadBytes, err := json.Marshal([]any{event, data})
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("42%d%s", ackID, string(payloadBytes))
	if err := c.write([]byte(msg)); err != nil {
		return nil, err
	}

	select {
	case res := <-respCh:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for client response")
	}
}

func (c *SocketClient) resolveAck(ackID int64, result any) {
	c.ackMu.Lock()
	defer c.ackMu.Unlock()
	if ch, ok := c.pendingAcks[ackID]; ok {
		select {
		case ch <- result:
		default:
		}
	}
}

// SocketHub manages rooms and active Socket.IO connections
type SocketHub struct {
	mu          sync.RWMutex
	clients     map[string]*SocketClient            // sid -> client
	rooms       map[string]map[string]*SocketClient // room -> (sid -> client)
	usagePool   map[string]map[string]int64         // model -> (sid -> timestamp)
	ydocUpdates map[string][]any                    // docID -> updates
}

var globalSocketHub = &SocketHub{
	clients:     make(map[string]*SocketClient),
	rooms:       make(map[string]map[string]*SocketClient),
	usagePool:   make(map[string]map[string]int64),
	ydocUpdates: make(map[string][]any),
}

func (h *SocketHub) register(c *SocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.sid] = c
}

func (h *SocketHub) unregister(c *SocketClient) {
	c.close()
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c.sid)
	for room := range c.rooms {
		if set, ok := h.rooms[room]; ok {
			delete(set, c.sid)
			if len(set) == 0 {
				delete(h.rooms, room)
			}
		}
	}
	// Clean from usage pool
	for _, sids := range h.usagePool {
		delete(sids, c.sid)
	}
}

func (h *SocketHub) getClient(sid string) *SocketClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[sid]
}

func (h *SocketHub) getClientsByUserID(userID string) []*SocketClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var matches []*SocketClient
	for _, client := range h.clients {
		if client.userID == userID {
			matches = append(matches, client)
		}
	}
	return matches
}

func (h *SocketHub) joinRoom(c *SocketClient, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.rooms == nil {
		c.rooms = make(map[string]bool)
	}
	c.rooms[room] = true
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[string]*SocketClient)
	}
	h.rooms[room][c.sid] = c
}

func (h *SocketHub) leaveRoom(c *SocketClient, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.rooms != nil {
		delete(c.rooms, room)
	}
	if set, ok := h.rooms[room]; ok {
		delete(set, c.sid)
		if len(set) == 0 {
			delete(h.rooms, room)
		}
	}
}

func (h *SocketHub) broadcastToRoom(room string, msg []byte, skipSid string) {
	h.mu.RLock()
	var targets []*SocketClient
	if set, ok := h.rooms[room]; ok {
		for sid, client := range set {
			if sid != skipSid {
				targets = append(targets, client)
			}
		}
	}
	h.mu.RUnlock()

	for _, client := range targets {
		_ = client.write(msg)
	}
}

func (h *SocketHub) emitToUsers(event string, data any, userIDs []string) {
	payload, err := json.Marshal([]any{event, data})
	if err != nil {
		return
	}
	msg := []byte("42" + string(payload))
	for _, uID := range userIDs {
		h.broadcastToRoom("user:"+uID, msg, "")
	}
}

func (h *SocketHub) enterRoomForUsers(room string, userIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, client := range h.clients {
		for _, uID := range userIDs {
			if client.userID == uID {
				if client.rooms == nil {
					client.rooms = make(map[string]bool)
				}
				client.rooms[room] = true
				if h.rooms[room] == nil {
					h.rooms[room] = make(map[string]*SocketClient)
				}
				h.rooms[room][client.sid] = client
			}
		}
	}
}

func (h *SocketHub) disconnectUserSessions(userID string) {
	h.mu.Lock()
	var targets []*SocketClient
	for _, client := range h.clients {
		if client.userID == userID {
			targets = append(targets, client)
		}
	}
	h.mu.Unlock()

	for _, client := range targets {
		h.unregister(client)
	}
}

func (h *SocketHub) getUserCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *SocketHub) recordUsage(model, sid string) {
	if model == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.usagePool[model] == nil {
		h.usagePool[model] = make(map[string]int64)
	}
	h.usagePool[model][sid] = time.Now().Unix()
}

func (h *SocketHub) getModelsInUse() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var models []string
	for m := range h.usagePool {
		models = append(models, m)
	}
	return models
}

func (h *SocketHub) getRoomSids(room string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var sids []string
	if set, ok := h.rooms[room]; ok {
		for sid := range set {
			sids = append(sids, sid)
		}
	}
	if sids == nil {
		sids = []string{}
	}
	return sids
}

func (h *SocketHub) appendYDocUpdate(docID string, update any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ydocUpdates[docID] = append(h.ydocUpdates[docID], update)
}

func (h *SocketHub) clearYDoc(docID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.ydocUpdates, docID)
}

func (h *SocketHub) getYDocState(docID string) []any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if updates, ok := h.ydocUpdates[docID]; ok {
		return updates
	}
	return []any{}
}

func (h *SocketHub) startPruner() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().Unix()
			h.mu.Lock()
			// 1. Prune usage pool (> 300s)
			for model, sids := range h.usagePool {
				for sid, ts := range sids {
					if now-ts > 300 {
						delete(sids, sid)
					}
				}
				if len(sids) == 0 {
					delete(h.usagePool, model)
				}
			}
			// 2. Prune dead client sessions (> 120s without heartbeat or ping)
			for sid, client := range h.clients {
				if now-client.lastSeenAt > 120 {
					delete(h.clients, sid)
					for room := range client.rooms {
						if set, ok := h.rooms[room]; ok {
							delete(set, sid)
							if len(set) == 0 {
								delete(h.rooms, room)
							}
						}
					}
					client.close()
				}
			}
			h.mu.Unlock()
		}
	}()
}

func init() {
	globalSocketHub.startPruner()
}

func normalizeDocumentID(docID string) string {
	docID = strings.TrimPrefix(docID, "doc_")
	docID = strings.TrimPrefix(docID, "/")
	return docID
}

// getFolderUnreadCounts computes hierarchical unread badge counts across folders
func (h *APIHandler) getFolderUnreadCounts(subdomain, userID string) map[string]int {
	folderList, err := h.db.GetOpenWebUIFolders(subdomain, userID)
	if err != nil {
		return map[string]int{}
	}

	parentByID := make(map[string]string)
	unreadCounts := make(map[string]int)
	for _, f := range folderList {
		parentByID[f.ID] = f.ParentID
		unreadCounts[f.ID] = 0
	}

	directUnread, err := h.db.CountOpenWebUIUnreadByFolder(subdomain, userID)
	if err == nil {
		for folderID, count := range directUnread {
			curr := folderID
			seen := make(map[string]bool)
			for curr != "" && !seen[curr] {
				seen[curr] = true
				if _, ok := unreadCounts[curr]; ok {
					unreadCounts[curr] += count
				}
				curr = parentByID[curr]
			}
		}
	}

	return unreadCounts
}

// EmitChatEvent emits an event (token chunks, title, status, etc.) to all user sessions
func (h *APIHandler) EmitChatEvent(subdomain, userID, chatID, messageID, eventType string, eventData any) {
	room := "user:" + userID
	payload, err := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"data": map[string]interface{}{
			"type": eventType,
			"data": eventData,
		},
	})
	if err == nil {
		globalSocketHub.broadcastToRoom(room, []byte(fmt.Sprintf(`42["events",%s]`, string(payload))), "")
	}
}

// GetEventEmitter provides an event emitter for chat completion streaming and updates
func (h *APIHandler) GetEventEmitter(subdomain, userID, chatID, messageID string, updateDB bool) func(eventData map[string]any) {
	if strings.HasPrefix(chatID, "channel:") {
		channelID := strings.TrimPrefix(chatID, "channel:")
		return h.MakeChannelEmitter(subdomain, channelID, messageID)
	}

	var mu sync.Mutex
	var accumChunk strings.Builder
	var lastDbWrite time.Time

	flushMessageChunk := func(force bool) {
		if !updateDB || messageID == "" || chatID == "" {
			return
		}
		mu.Lock()
		if accumChunk.Len() == 0 {
			mu.Unlock()
			return
		}
		if !force && time.Since(lastDbWrite) < 1500*time.Millisecond {
			mu.Unlock()
			return
		}
		toFlush := accumChunk.String()
		accumChunk.Reset()
		lastDbWrite = time.Now()
		mu.Unlock()

		_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
			curr, _ := msg["content"].(string)
			msg["content"] = curr + toFlush
			return msg
		})
	}

	return func(eventData map[string]any) {
		if eventData == nil {
			return
		}
		eventType, _ := eventData["type"].(string)

		// Broadcast event to user room
		room := "user:" + userID
		payload, err := json.Marshal(map[string]any{
			"chat_id":    chatID,
			"message_id": messageID,
			"data":       eventData,
		})
		if err == nil {
			globalSocketHub.broadcastToRoom(room, []byte(fmt.Sprintf(`42["events",%s]`, string(payload))), "")
		}

		if !updateDB || messageID == "" || chatID == "" {
			return
		}

		dataMap, _ := eventData["data"].(map[string]any)

		switch eventType {
		case "status":
			if dataMap != nil {
				_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
					msg["status"] = dataMap
					return msg
				})
			}
		case "message":
			if dataMap != nil {
				chunk, _ := dataMap["content"].(string)
				mu.Lock()
				accumChunk.WriteString(chunk)
				mu.Unlock()
				flushMessageChunk(false)
			}
		case "replace":
			flushMessageChunk(true)
			if dataMap != nil {
				content, _ := dataMap["content"].(string)
				_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
					msg["content"] = content
					return msg
				})
			}
		case "done":
			flushMessageChunk(true)
			_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
				msg["done"] = true
				return msg
			})
		case "embeds":
			if dataMap != nil {
				embeds, _ := dataMap["embeds"].([]any)
				replace, _ := dataMap["replace"].(bool)
				_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
					if replace || msg["embeds"] == nil {
						msg["embeds"] = embeds
					} else if existing, ok := msg["embeds"].([]any); ok {
						msg["embeds"] = append(existing, embeds...)
					}
					return msg
				})
			}
		case "files":
			if dataMap != nil {
				files, _ := dataMap["files"].([]any)
				_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
					if msg["files"] == nil {
						msg["files"] = files
					} else if existing, ok := msg["files"].([]any); ok {
						msg["files"] = append(existing, files...)
					}
					return msg
				})
			}
		case "source", "citation":
			if dataMap != nil && dataMap["type"] != nil {
				_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, messageID, func(msg map[string]interface{}) map[string]interface{} {
					var sources []any
					if sList, ok := msg["sources"].([]any); ok {
						sources = sList
					}
					sources = append(sources, dataMap)
					msg["sources"] = sources
					return msg
				})
			}
		}
	}
}

// MakeChannelEmitter translates chat completions into throttled channel message updates
func (h *APIHandler) MakeChannelEmitter(subdomain, channelID, messageID string) func(eventData map[string]any) {
	var mu sync.Mutex
	var lastEmitAt float64
	var output []any
	throttleInterval := 0.15 // 150ms

	return func(eventData map[string]any) {
		if eventData == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()

		eventType, _ := eventData["type"].(string)
		data, _ := eventData["data"].(map[string]any)
		if data == nil {
			data = make(map[string]any)
		}

		now := float64(time.Now().UnixNano()) / 1e9

		if eventType == "chat:completion" {
			outList, _ := data["output"].([]any)
			if outList != nil {
				output = outList
			}
			content, _ := data["content"].(string)
			done, _ := data["done"].(bool)
			flush, _ := data["flush"].(bool)

			if content == "" && len(output) == 0 && !done {
				return
			}

			if done || flush || (now-lastEmitAt >= throttleInterval) {
				lastEmitAt = now
				updatePayload, _ := json.Marshal(map[string]any{
					"channel_id": channelID,
					"message_id": messageID,
					"data": map[string]any{
						"type": "message:update",
						"data": map[string]any{
							"id":         messageID,
							"channel_id": channelID,
							"content":    content,
							"done":       done,
							"output":     output,
						},
					},
				})
				globalSocketHub.broadcastToRoom("channel:"+channelID, []byte(fmt.Sprintf(`42["events:channel",%s]`, string(updatePayload))), "")
			}
		} else if eventType == "chat:message:error" {
			errMap, _ := data["error"].(map[string]any)
			errContent := "An error occurred"
			if errMap != nil {
				if msg, ok := errMap["content"].(string); ok && msg != "" {
					errContent = msg
				}
			}
			updatePayload, _ := json.Marshal(map[string]any{
				"channel_id": channelID,
				"message_id": messageID,
				"data": map[string]any{
					"type": "message:update",
					"data": map[string]any{
						"id":         messageID,
						"channel_id": channelID,
						"content":    "Error: " + errContent,
						"done":       true,
					},
				},
			})
			globalSocketHub.broadcastToRoom("channel:"+channelID, []byte(fmt.Sprintf(`42["events:channel",%s]`, string(updatePayload))), "")
		}
	}
}

// GetEventCall provides an RPC caller to send events to a specific client session and await response
func (h *APIHandler) GetEventCall(subdomain, userID, sessionID, chatID, messageID string) func(eventData map[string]any) (any, error) {
	client := globalSocketHub.getClient(sessionID)
	if client == nil || client.userID != userID {
		return func(eventData map[string]any) (any, error) {
			return map[string]any{"error": "Client session disconnected."}, nil
		}
	}

	return func(eventData map[string]any) (any, error) {
		req := map[string]any{
			"chat_id":    chatID,
			"message_id": messageID,
			"data":       eventData,
		}
		// 30-second timeout matching WEBSOCKET_EVENT_CALLER_TIMEOUT
		resp, err := client.Call("events", req, 30*time.Second)
		if err != nil {
			return map[string]any{"error": "Event call timed out. The browser tab may be inactive or closed."}, nil
		}
		return resp, nil
	}
}

// parseSocketIOEvent parses a Socket.IO message like: 42["user-join",{"auth":...}] or 421[...]
func parseSocketIOEvent(msg string) (ackID string, eventName string, eventData map[string]interface{}) {
	if !strings.HasPrefix(msg, "42") {
		return "", "", nil
	}
	rest := msg[2:]
	idx := strings.Index(rest, "[")
	if idx < 0 {
		return "", "", nil
	}
	ackID = rest[:idx]
	jsonPart := rest[idx:]

	var arr []interface{}
	if err := json.Unmarshal([]byte(jsonPart), &arr); err == nil && len(arr) > 0 {
		if name, ok := arr[0].(string); ok {
			eventName = name
		}
		if len(arr) > 1 {
			if dataMap, ok := arr[1].(map[string]interface{}); ok {
				eventData = dataMap
			}
		}
	}
	return ackID, eventName, eventData
}

// parseSocketIOAck parses a Socket.IO ack response like: 431[{"ok":true}]
func parseSocketIOAck(msg string) (int64, any) {
	if !strings.HasPrefix(msg, "43") {
		return 0, nil
	}
	rest := msg[2:]
	idx := strings.Index(rest, "[")
	if idx < 0 {
		return 0, nil
	}
	var ackID int64
	_, _ = fmt.Sscanf(rest[:idx], "%d", &ackID)
	jsonPart := rest[idx:]
	var arr []any
	if err := json.Unmarshal([]byte(jsonPart), &arr); err == nil && len(arr) > 0 {
		return ackID, arr[0]
	}
	return ackID, nil
}

// handleOpenWebUISocketIO handles WebSocket and Engine.IO polling on /ws/socket.io/
func (h *APIHandler) handleOpenWebUISocketIO(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		sid := "sio_" + hex.EncodeToString(randBytes)

		subdomain := h.resolveSubdomain(r)
		client := &SocketClient{
			conn:        conn,
			sid:         sid,
			subdomain:   subdomain,
			userID:      "usr_guest",
			userName:    "Guest",
			lastSeenAt:  time.Now().Unix(),
			rooms:       make(map[string]bool),
			send:        make(chan []byte, 256),
			pendingAcks: make(map[int64]chan any),
		}

		globalSocketHub.register(client)
		go client.writePump()
		defer func() {
			// Leave doc rooms and notify if any
			for rName := range client.rooms {
				if strings.HasPrefix(rName, "doc_") {
					docID := strings.TrimPrefix(rName, "doc_")
					leavePayload, _ := json.Marshal(map[string]interface{}{
						"document_id": docID,
						"user_id":     client.userID,
					})
					leaveEvt := fmt.Sprintf(`42["ydoc:user:left",%s]`, string(leavePayload))
					globalSocketHub.broadcastToRoom(rName, []byte(leaveEvt), client.sid)
				}
			}
			globalSocketHub.unregister(client)
		}()

		// Engine.IO v4 Handshake packet: pingInterval 25s, pingTimeout 60s
		handshake := fmt.Sprintf(`0{"sid":"%s","upgrades":[],"pingInterval":25000,"pingTimeout":60000}`, sid)
		if err := client.writeDirect([]byte(handshake)); err != nil {
			return
		}

		conn.SetReadLimit(10 * 1024 * 1024)
		_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// Extend read deadline on ANY message from client
			_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
			client.lastSeenAt = time.Now().Unix()

			s := string(msg)
			if s == "2" { // Engine.IO ping -> reply pong directly
				_ = client.writeDirect([]byte("3"))
			} else if strings.HasPrefix(s, "40") { // Socket.IO CONNECT
				connectResp := fmt.Sprintf(`40{"sid":"%s"}`, sid)
				_ = client.write([]byte(connectResp))
			} else if strings.HasPrefix(s, "43") { // Socket.IO ACK response from client
				ackID, result := parseSocketIOAck(s)
				if ackID > 0 {
					client.resolveAck(ackID, result)
				}
			} else if strings.HasPrefix(s, "42") { // Socket.IO EVENT
				ackID, eventName, eventData := parseSocketIOEvent(s)
				if eventData == nil {
					eventData = make(map[string]interface{})
				}

				switch eventName {
				case "user-join":
					var token string
					if authMap, ok := eventData["auth"].(map[string]interface{}); ok {
						if t, ok := authMap["token"].(string); ok {
							token = t
						}
					}
					if token == "" {
						if c, err := r.Cookie("token"); err == nil && c.Value != "" {
							token = c.Value
						} else if c, err := r.Cookie("zcdns_user_token"); err == nil && c.Value != "" {
							token = c.Value
						}
					}
					u := h.resolveUserFromToken(subdomain, token)
					client.userID = u.ID
					client.userName = u.Name
					client.role = u.Role
					client.email = u.Email
					client.lastSeenAt = time.Now().Unix()

					globalSocketHub.joinRoom(client, "user:"+u.ID)
					globalSocketHub.joinRoom(client, "user_all")
					globalSocketHub.joinRoom(client, "channel:general")
					if u.ID != "" {
						globalSocketHub.joinRoom(client, "channel:"+u.ID)
					}

					// Reply with ACK if requested
					if ackID != "" {
						ackPayload, _ := json.Marshal([]any{map[string]interface{}{
							"id":                u.ID,
							"name":              u.Name,
							"role":              u.Role,
							"email":             u.Email,
							"profile_image_url": u.ProfileImageURL,
						}})
						_ = client.write([]byte(fmt.Sprintf("43%s%s", ackID, string(ackPayload))))
					}
					// Send user-count event
					countEvt := fmt.Sprintf(`42["user-count",{"count":%d}]`, globalSocketHub.getUserCount())
					_ = client.write([]byte(countEvt))

				case "heartbeat":
					client.lastSeenAt = time.Now().Unix()
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "usage":
					if model, ok := eventData["model"].(string); ok && model != "" {
						globalSocketHub.recordUsage(model, client.sid)
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "join-channel", "join-channels":
					channelID, _ := eventData["channel_id"].(string)
					if channelID == "" {
						channelID = "general"
					}
					globalSocketHub.joinRoom(client, "channel:"+channelID)
					if client.userID != "" {
						globalSocketHub.joinRoom(client, "channel:"+client.userID)
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "join-note":
					if noteID, ok := eventData["note_id"].(string); ok && noteID != "" {
						globalSocketHub.joinRoom(client, "note:"+noteID)
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "events:channel":
					channelID, _ := eventData["channel_id"].(string)
					room := "channel:" + channelID
					if ed, ok := eventData["data"].(map[string]interface{}); ok {
						eventType, _ := ed["type"].(string)
						if eventType == "typing" {
							broadcastPayload, _ := json.Marshal(map[string]interface{}{
								"channel_id": channelID,
								"message_id": eventData["message_id"],
								"data":       ed,
								"user": map[string]string{
									"id":   client.userID,
									"name": client.userName,
								},
							})
							evtMsg := fmt.Sprintf(`42["events:channel",%s]`, string(broadcastPayload))
							globalSocketHub.broadcastToRoom(room, []byte(evtMsg), client.sid)
						} else if eventType == "last_read_at" {
							// Channel read ack
						}
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "events:chat":
					chatID, _ := eventData["chat_id"].(string)
					if ed, ok := eventData["data"].(map[string]interface{}); ok {
						if ed["type"] == "last_read_at" {
							lastReadAt, wasUnread, err := h.db.UpdateOpenWebUIChatLastReadAt(subdomain, client.userID, chatID)
							if err == nil {
								responseData := map[string]interface{}{
									"chat_id":      chatID,
									"last_read_at": lastReadAt,
								}
								if wasUnread {
									responseData["folder_unread_counts"] = h.getFolderUnreadCounts(subdomain, client.userID)
								}
								respPayload, _ := json.Marshal(map[string]interface{}{
									"chat_id": chatID,
									"data": map[string]interface{}{
										"type": "chat:list",
										"data": responseData,
									},
								})
								globalSocketHub.broadcastToRoom("user:"+client.userID, []byte(fmt.Sprintf(`42["events",%s]`, string(respPayload))), "")
							}
						}
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:document:join":
					rawDocID, _ := eventData["document_id"].(string)
					docID := normalizeDocumentID(rawDocID)
					if docID != "" {
						room := "doc_" + docID
						globalSocketHub.joinRoom(client, room)

						// 1. Notify existing users in document
						joinedPayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"user_id":     client.userID,
							"user_name":   client.userName,
							"user_color":  "#3b82f6",
						})
						joinedEvt := fmt.Sprintf(`42["ydoc:user:joined",%s]`, string(joinedPayload))
						globalSocketHub.broadcastToRoom(room, []byte(joinedEvt), client.sid)

						// 2. Send current document state to the joined user
						statePayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"state":       globalSocketHub.getYDocState(docID),
							"sessions":    globalSocketHub.getRoomSids(room),
						})
						stateEvt := fmt.Sprintf(`42["ydoc:document:state",%s]`, string(statePayload))
						_ = client.write([]byte(stateEvt))
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:document:state":
					rawDocID, _ := eventData["document_id"].(string)
					docID := normalizeDocumentID(rawDocID)
					if docID != "" {
						room := "doc_" + docID
						statePayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"state":       globalSocketHub.getYDocState(docID),
							"sessions":    globalSocketHub.getRoomSids(room),
						})
						stateEvt := fmt.Sprintf(`42["ydoc:document:state",%s]`, string(statePayload))
						_ = client.write([]byte(stateEvt))
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:document:update":
					rawDocID, _ := eventData["document_id"].(string)
					docID := normalizeDocumentID(rawDocID)
					update := eventData["update"]
					if docID != "" && update != nil {
						globalSocketHub.appendYDocUpdate(docID, update)
						room := "doc_" + docID
						updatePayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"user_id":     client.userID,
							"update":      update,
							"socket_id":   client.sid,
						})
						updateEvt := fmt.Sprintf(`42["ydoc:document:update",%s]`, string(updatePayload))
						globalSocketHub.broadcastToRoom(room, []byte(updateEvt), client.sid)
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:document:leave":
					rawDocID, _ := eventData["document_id"].(string)
					docID := normalizeDocumentID(rawDocID)
					if docID != "" {
						room := "doc_" + docID
						globalSocketHub.leaveRoom(client, room)
						leavePayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"user_id":     client.userID,
						})
						leaveEvt := fmt.Sprintf(`42["ydoc:user:left",%s]`, string(leavePayload))
						globalSocketHub.broadcastToRoom(room, []byte(leaveEvt), client.sid)

						if len(globalSocketHub.getRoomSids(room)) == 0 {
							globalSocketHub.clearYDoc(docID)
						}
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:awareness:update":
					rawDocID, _ := eventData["document_id"].(string)
					docID := normalizeDocumentID(rawDocID)
					update := eventData["update"]
					if docID != "" && update != nil {
						room := "doc_" + docID
						awarePayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"user_id":     client.userID,
							"update":      update,
						})
						awareEvt := fmt.Sprintf(`42["ydoc:awareness:update",%s]`, string(awarePayload))
						globalSocketHub.broadcastToRoom(room, []byte(awareEvt), client.sid)
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				default:
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}
				}
			}
		}
		return
	}

	// Polling fallback transport
	if setOWUCors(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	if r.Method == http.MethodGet {
		w.Write([]byte(`0{"sid":"zcdns","upgrades":["websocket"],"pingInterval":25000,"pingTimeout":20000}`))
		return
	}
	if r.Method == http.MethodPost {
		w.Write([]byte("ok"))
		return
	}
}
