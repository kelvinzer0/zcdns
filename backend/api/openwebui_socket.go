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
	conn       *websocket.Conn
	sid        string
	subdomain  string
	userID     string
	userName   string
	lastSeenAt int64
	rooms      map[string]bool
	writeMu    sync.Mutex
}

func (c *SocketClient) write(msg []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, msg)
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

func (h *SocketHub) getYDocState(docID string) []any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if updates, ok := h.ydocUpdates[docID]; ok {
		return updates
	}
	return []any{}
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
			conn:       conn,
			sid:        sid,
			subdomain:  subdomain,
			userID:     "usr_guest",
			userName:   "Guest",
			lastSeenAt: time.Now().Unix(),
			rooms:      make(map[string]bool),
		}

		globalSocketHub.register(client)
		defer func() {
			// Leave doc rooms and notify if any
			for rName := range client.rooms {
				if strings.HasPrefix(rName, "doc_") {
					docID := strings.TrimPrefix(rName, "doc_")
					leaveEvt := fmt.Sprintf(`42["ydoc:user:left",{"document_id":"%s","user_id":"%s"}]`, docID, client.userID)
					globalSocketHub.broadcastToRoom(rName, []byte(leaveEvt), client.sid)
				}
			}
			globalSocketHub.unregister(client)
		}()

		// Engine.IO v4 Handshake packet
		handshake := fmt.Sprintf(`0{"sid":"%s","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`, sid)
		if err := client.write([]byte(handshake)); err != nil {
			return
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			s := string(msg)
			if s == "2" { // Engine.IO ping -> reply pong
				_ = client.write([]byte("3"))
			} else if strings.HasPrefix(s, "40") { // Socket.IO CONNECT
				connectResp := fmt.Sprintf(`40{"sid":"%s"}`, sid)
				_ = client.write([]byte(connectResp))
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
					client.lastSeenAt = time.Now().Unix()

					globalSocketHub.joinRoom(client, "user:"+u.ID)
					globalSocketHub.joinRoom(client, "user_all")

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

				case "join-channels":
					globalSocketHub.joinRoom(client, "channel:general")
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
						if ed["type"] == "typing" {
							broadcastPayload, _ := json.Marshal(map[string]interface{}{
								"channel_id": channelID,
								"data":       ed,
								"user": map[string]string{
									"id":   client.userID,
									"name": client.userName,
								},
							})
							evtMsg := fmt.Sprintf(`42["events:channel",%s]`, string(broadcastPayload))
							globalSocketHub.broadcastToRoom(room, []byte(evtMsg), client.sid)
						}
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "events:chat":
					chatID, _ := eventData["chat_id"].(string)
					if ed, ok := eventData["data"].(map[string]interface{}); ok {
						if ed["type"] == "last_read_at" {
							now := time.Now().Unix()
							respEvt := fmt.Sprintf(`42["events",{"chat_id":"%s","data":{"type":"chat:list","data":{"chat_id":"%s","last_read_at":%d}}}]`, chatID, chatID, now)
							globalSocketHub.broadcastToRoom("user:"+client.userID, []byte(respEvt), "")
						}
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:document:join":
					docID, _ := eventData["document_id"].(string)
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
					docID, _ := eventData["document_id"].(string)
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
					docID, _ := eventData["document_id"].(string)
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
					docID, _ := eventData["document_id"].(string)
					if docID != "" {
						room := "doc_" + docID
						globalSocketHub.leaveRoom(client, room)
						leavePayload, _ := json.Marshal(map[string]interface{}{
							"document_id": docID,
							"user_id":     client.userID,
						})
						leaveEvt := fmt.Sprintf(`42["ydoc:user:left",%s]`, string(leavePayload))
						globalSocketHub.broadcastToRoom(room, []byte(leaveEvt), client.sid)
					}
					if ackID != "" {
						_ = client.write([]byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "ydoc:awareness:update":
					docID, _ := eventData["document_id"].(string)
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
