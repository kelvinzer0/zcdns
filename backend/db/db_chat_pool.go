package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type chatDBEntry struct {
	db         *sql.DB
	path       string
	lastAccess time.Time
}

// ChatDBPool manages per-conversation SQLite databases (SQLite-per-chat).
// Each conversation has its own isolated file: {baseDir}/{subdomain}/{userID}/{chatID}.db
// This eliminates write-lock contention across concurrent chat sessions and users.
type ChatDBPool struct {
	baseDir string
	mu      sync.Mutex
	pools   map[string]*chatDBEntry
	closing bool
}

// NewChatDBPool creates and initializes the chat DB pool.
func NewChatDBPool(baseDir string) *ChatDBPool {
	if baseDir == "" {
		baseDir = filepath.Join("data", "chats")
	}
	_ = os.MkdirAll(baseDir, 0755)

	pool := &ChatDBPool{
		baseDir: baseDir,
		pools:   make(map[string]*chatDBEntry),
	}

	// Idle connection reaper: closes chat DBs unused for > 5 minutes
	go pool.reaperLoop()

	return pool
}

func sanitizePathPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if res == "" || res == "." || res == ".." {
		return "default"
	}
	return res
}

func (p *ChatDBPool) makeKey(subdomain, userID, chatID string) string {
	return fmt.Sprintf("%s:%s:%s", sanitizePathPart(subdomain), sanitizePathPart(userID), sanitizePathPart(chatID))
}

func (p *ChatDBPool) makePath(subdomain, userID, chatID string) string {
	sub := sanitizePathPart(subdomain)
	usr := sanitizePathPart(userID)
	cht := sanitizePathPart(chatID)
	return filepath.Join(p.baseDir, sub, usr, cht+".db")
}

// GetChatDB retrieves or opens an isolated SQLite database for a conversation.
func (p *ChatDBPool) GetChatDB(subdomain, userID, chatID string) (*sql.DB, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closing {
		return nil, "", fmt.Errorf("chat pool is closing")
	}

	key := p.makeKey(subdomain, userID, chatID)
	if entry, exists := p.pools[key]; exists {
		entry.lastAccess = time.Now()
		return entry.db, entry.path, nil
	}

	dbPath := p.makePath(subdomain, userID, chatID)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create chat directory: %w", err)
	}

	// Open isolated SQLite with WAL mode & busy timeout
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open chat database %s: %w", dbPath, err)
	}

	// WAL mode allows concurrent readers while one writer is writing
	_, _ = conn.Exec("PRAGMA journal_mode = WAL;")
	_, _ = conn.Exec("PRAGMA busy_timeout = 5000;")
	_, _ = conn.Exec("PRAGMA synchronous = NORMAL;")

	schema := `
	CREATE TABLE IF NOT EXISTS chat_session (
		id TEXT PRIMARY KEY,
		subdomain TEXT NOT NULL,
		user_id TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		folder_id TEXT,
		pinned INTEGER NOT NULL DEFAULT 0,
		archived INTEGER NOT NULL DEFAULT 0,
		chat_json TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0,
		last_read_at INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		parent_id TEXT,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		meta_json TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_msg_parent ON messages(parent_id);
	`
	if _, err := conn.Exec(schema); err != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("failed to initialize chat session schema: %w", err)
	}

	p.pools[key] = &chatDBEntry{
		db:         conn,
		path:       dbPath,
		lastAccess: time.Now(),
	}

	return conn, dbPath, nil
}

// CloseChatDB closes and removes a single chat database from the pool.
func (p *ChatDBPool) CloseChatDB(subdomain, userID, chatID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.makeKey(subdomain, userID, chatID)
	if entry, exists := p.pools[key]; exists {
		_ = entry.db.Close()
		delete(p.pools, key)
	}
}

// DeleteChatDB closes and deletes the SQLite file and WAL journals for a chat.
func (p *ChatDBPool) DeleteChatDB(subdomain, userID, chatID string) error {
	p.CloseChatDB(subdomain, userID, chatID)

	dbPath := p.makePath(subdomain, userID, chatID)
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	return nil
}

// DeleteUserChats closes and deletes all chat databases for a user.
func (p *ChatDBPool) DeleteUserChats(subdomain, userID string) error {
	p.mu.Lock()
	prefix := fmt.Sprintf("%s:%s:", sanitizePathPart(subdomain), sanitizePathPart(userID))
	for key, entry := range p.pools {
		if strings.HasPrefix(key, prefix) {
			_ = entry.db.Close()
			delete(p.pools, key)
		}
	}
	p.mu.Unlock()

	userDir := filepath.Join(p.baseDir, sanitizePathPart(subdomain), sanitizePathPart(userID))
	return os.RemoveAll(userDir)
}

func (p *ChatDBPool) reaperLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closing {
			p.mu.Unlock()
			return
		}
		cutoff := time.Now().Add(-5 * time.Minute)
		for key, entry := range p.pools {
			if entry.lastAccess.Before(cutoff) {
				_ = entry.db.Close()
				delete(p.pools, key)
			}
		}
		p.mu.Unlock()
	}
}

// CloseAll closes all open database connections in the pool.
func (p *ChatDBPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closing = true
	for key, entry := range p.pools {
		_ = entry.db.Close()
		delete(p.pools, key)
	}
}

// SyncMessagesTree parses chatJSON and stores all messages in the messages table (DAG tree).
func (p *ChatDBPool) SyncMessagesTree(db *sql.DB, chatJSON string) {
	if db == nil || strings.TrimSpace(chatJSON) == "" || chatJSON == "{}" {
		return
	}

	var chatObj map[string]interface{}
	if err := json.Unmarshal([]byte(chatJSON), &chatObj); err != nil {
		return
	}

	target := chatObj
	if inner, ok := chatObj["chat"].(map[string]interface{}); ok && inner != nil {
		target = inner
	}

	history, _ := target["history"].(map[string]interface{})
	if history == nil {
		return
	}

	messages, _ := history["messages"].(map[string]interface{})
	if len(messages) == 0 {
		return
	}

	now := time.Now().Unix()
	for msgID, rawMsg := range messages {
		msgMap, ok := rawMsg.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msgMap["role"].(string)
		content := ""
		if c, ok := msgMap["content"].(string); ok {
			content = c
		}
		parentID, _ := msgMap["parentId"].(string)
		metaBytes, _ := json.Marshal(msgMap)

		ts := now
		if t, ok := msgMap["timestamp"].(float64); ok && t > 0 {
			ts = int64(t)
		}

		_, _ = db.Exec(`
			INSERT INTO messages (id, parent_id, role, content, meta_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				content = CASE WHEN excluded.content != '' THEN excluded.content ELSE messages.content END,
				meta_json = excluded.meta_json,
				updated_at = excluded.updated_at
		`, msgID, parentID, role, content, string(metaBytes), ts, now)
	}
}
