package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func InitDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Performance tuning pragmas
	_, _ = conn.Exec("PRAGMA journal_mode = WAL;")
	_, _ = conn.Exec("PRAGMA busy_timeout = 5000;")
	_, _ = conn.Exec("PRAGMA synchronous = NORMAL;")

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		subdomain TEXT UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS records (
		id TEXT PRIMARY KEY,
		subdomain TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		ttl INTEGER NOT NULL DEFAULT 60,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS requests (
		id TEXT PRIMARY KEY,
		subdomain TEXT NOT NULL,
		qname TEXT NOT NULL,
		qtype TEXT NOT NULL,
		client_ip TEXT NOT NULL,
		rcode TEXT NOT NULL,
		answers TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS parental_configs (
		subdomain TEXT PRIMARY KEY,
		enabled INTEGER DEFAULT 1,
		block_adult INTEGER DEFAULT 1,
		block_gambling INTEGER DEFAULT 1,
		block_malware INTEGER DEFAULT 1,
		block_ads INTEGER DEFAULT 1,
		block_social INTEGER DEFAULT 0,
		block_gaming INTEGER DEFAULT 0,
		enforce_safesearch INTEGER DEFAULT 1,
		block_mode TEXT DEFAULT '0.0.0.0',
		custom_blocked TEXT DEFAULT '[]',
		custom_allowed TEXT DEFAULT '[]',
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS abuse_reports (
		id TEXT PRIMARY KEY,
		reporter_name TEXT NOT NULL,
		reporter_email TEXT NOT NULL,
		abuse_type TEXT NOT NULL,
		subdomain TEXT NOT NULL,
		description TEXT NOT NULL,
		evidence TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		admin_notes TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS blocked_subdomains (
		subdomain TEXT PRIMARY KEY,
		reason TEXT NOT NULL,
		blocked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS vault_repos (
		id TEXT PRIMARY KEY,
		subdomain TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS vault_branches (
		id TEXT PRIMARY KEY,
		repo_id TEXT NOT NULL,
		name TEXT NOT NULL,
		head_commit TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(repo_id, name)
	);

	CREATE TABLE IF NOT EXISTS vault_commits (
		id TEXT PRIMARY KEY,
		repo_id TEXT NOT NULL,
		hash TEXT UNIQUE NOT NULL,
		parent_hash TEXT,
		message TEXT NOT NULL,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		author TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS vault_kv_pairs (
		id TEXT PRIMARY KEY,
		commit_hash TEXT NOT NULL,
		key_name TEXT NOT NULL,
		encrypted_value TEXT NOT NULL,
		UNIQUE(commit_hash, key_name)
	);

	CREATE TABLE IF NOT EXISTS vault_auth_requests (
		device_code TEXT PRIMARY KEY,
		user_code TEXT UNIQUE NOT NULL,
		subdomain TEXT,
		token TEXT,
		status TEXT DEFAULT 'pending',
		expires_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_vault_auth_user_code ON vault_auth_requests(user_code);

	CREATE TABLE IF NOT EXISTS ai_router_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subdomain TEXT NOT NULL UNIQUE,
		input_format TEXT NOT NULL DEFAULT 'auto',
		output_format TEXT NOT NULL DEFAULT 'openai',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ai_router_connections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subdomain TEXT NOT NULL,
		provider TEXT NOT NULL,
		name TEXT NOT NULL,
		api_key TEXT NOT NULL,
		base_url TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active',
		rate_limited_until TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ai_router_combos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subdomain TEXT NOT NULL,
		name TEXT NOT NULL,
		strategy TEXT NOT NULL DEFAULT 'fallback',
		models_json TEXT NOT NULL DEFAULT '[]',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(subdomain, name)
	);

	CREATE TABLE IF NOT EXISTS ai_router_aliases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subdomain TEXT NOT NULL,
		alias_name TEXT NOT NULL,
		target_model TEXT NOT NULL,
		context_size INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(subdomain, alias_name)
	);

	CREATE TABLE IF NOT EXISTS ai_router_user_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subdomain TEXT NOT NULL,
		name TEXT NOT NULL,
		key_value TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(subdomain, key_value)
	);

	CREATE INDEX IF NOT EXISTS idx_records_subdomain ON records(subdomain);
	CREATE INDEX IF NOT EXISTS idx_records_lookup ON records(subdomain, name, type);
	CREATE INDEX IF NOT EXISTS idx_requests_subdomain ON requests(subdomain);
	CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at);
	CREATE INDEX IF NOT EXISTS idx_abuse_status ON abuse_reports(status);
	CREATE INDEX IF NOT EXISTS idx_abuse_subdomain ON abuse_reports(subdomain);
	CREATE INDEX IF NOT EXISTS idx_airouter_conn ON ai_router_connections(subdomain, provider, status);
	CREATE INDEX IF NOT EXISTS idx_airouter_user_keys ON ai_router_user_keys(subdomain, key_value);
	`

	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Migrations for existing databases
	_, _ = conn.Exec("ALTER TABLE ai_router_aliases ADD COLUMN context_size INTEGER NOT NULL DEFAULT 0;")

	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}
