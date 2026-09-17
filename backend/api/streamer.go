package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"zcdns-backend/db"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dev/dashboard
	},
}

type Client struct {
	hub       *StreamHub
	conn      *websocket.Conn
	subdomain string
	send      chan []byte
}

type StreamHub struct {
	// Map of subdomain -> map[*Client]bool
	clients    map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan *SubdomainMessage
	mu         sync.RWMutex
}

type SubdomainMessage struct {
	Subdomain string
	Data      []byte
}

func NewStreamHub() *StreamHub {
	return &StreamHub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *SubdomainMessage, 256),
	}
}

func (h *StreamHub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.subdomain]; !ok {
				h.clients[client.subdomain] = make(map[*Client]bool)
			}
			h.clients[client.subdomain][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if subClients, ok := h.clients[client.subdomain]; ok {
				if _, exists := subClients[client]; exists {
					delete(subClients, client)
					close(client.send)
					if len(subClients) == 0 {
						delete(h.clients, client.subdomain)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			if subClients, ok := h.clients[msg.Subdomain]; ok {
				for client := range subClients {
					select {
					case client.send <- msg.Data:
					default:
						close(client.send)
						delete(subClients, client)
					}
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			// Send heartbeat ping to all connected clients
			h.mu.RLock()
			for _, subClients := range h.clients {
				for client := range subClients {
					_ = client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					_ = client.conn.WriteMessage(websocket.PingMessage, nil)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Implements dns.QueryBroadcaster
func (h *StreamHub) BroadcastRequest(subdomain string, req *db.RequestLog) {
	data, err := json.Marshal(req)
	if err != nil {
		return
	}
	h.broadcast <- &SubdomainMessage{
		Subdomain: subdomain,
		Data:      data,
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *StreamHub) HandleWebSocket(w http.ResponseWriter, r *http.Request, subdomain string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:       h,
		conn:      conn,
		subdomain: subdomain,
		send:      make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}
