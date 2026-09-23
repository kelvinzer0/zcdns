package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

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

		// Engine.IO v4 Handshake packet
		handshake := `0{"sid":"zcdns","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(handshake)); err != nil {
			return
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			s := string(msg)
			if s == "2" { // Engine.IO ping -> reply pong
				_ = conn.WriteMessage(websocket.TextMessage, []byte("3"))
			} else if strings.HasPrefix(s, "40") { // Socket.IO CONNECT
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`40{"sid":"zcdns"}`))
			} else if strings.HasPrefix(s, "42") { // Socket.IO EVENT
				ackID, eventName, eventData := parseSocketIOEvent(s)
				switch eventName {
				case "user-join":
					// Resolve dynamic user from auth payload or cookie
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
					subdomain := h.resolveSubdomain(r)
					u := h.resolveUserFromToken(subdomain, token)

					// Reply with ACK if requested
					if ackID != "" {
						ackPayload, _ := json.Marshal([]any{map[string]interface{}{
							"id":                u.ID,
							"name":              u.Name,
							"role":              u.Role,
							"email":             u.Email,
							"profile_image_url": u.ProfileImageURL,
						}})
						ack := fmt.Sprintf("43%s%s", ackID, string(ackPayload))
						_ = conn.WriteMessage(websocket.TextMessage, []byte(ack))
					}
					// Send user-count event
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`42["user-count",{"count":1}]`))

				case "heartbeat":
					if ackID != "" {
						_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				case "events:chat":
					if ed, ok := eventData["data"].(map[string]interface{}); ok {
						if ed["type"] == "last_read_at" {
							chatID, _ := eventData["chat_id"].(string)
							now := time.Now().Unix()
							respEvt := fmt.Sprintf(`42["events",{"chat_id":"%s","data":{"type":"chat:list","data":{"chat_id":"%s","last_read_at":%d}}}]`, chatID, chatID, now)
							_ = conn.WriteMessage(websocket.TextMessage, []byte(respEvt))
						}
					}
					if ackID != "" {
						_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`43%s[true]`, ackID)))
					}

				default:
					if ackID != "" {
						_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`43%s[{}]`, ackID)))
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
