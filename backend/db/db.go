package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type Record struct {
	ID        string    `json:"id"`
	Subdomain string    `json:"subdomain"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	TTL       int       `json:"ttl"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RequestLog struct {
	ID        string    `json:"id"`
	Subdomain string    `json:"subdomain"`
	QName     string    `json:"qname"`
	QType     string    `json:"qtype"`
	ClientIP  string    `json:"client_ip"`
	RCode     string    `json:"rcode"`
	Answers   []string  `json:"answers"`
	CreatedAt time.Time `json:"created_at"`
}

type UserSession struct {
	ID         string    `json:"id"`
	Subdomain  string    `json:"subdomain"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

type ParentalConfig struct {
	Subdomain         string    `json:"subdomain"`
	Enabled           bool      `json:"enabled"`
	BlockAdult        bool      `json:"block_adult"`
	BlockGambling     bool      `json:"block_gambling"`
	BlockMalware      bool      `json:"block_malware"`
	BlockAds          bool      `json:"block_ads"`
	BlockSocial       bool      `json:"block_social"`
	BlockGaming       bool      `json:"block_gaming"`
	EnforceSafeSearch bool      `json:"enforce_safesearch"`
	BlockMode         string    `json:"block_mode"` // "0.0.0.0" or "NXDOMAIN"
	CustomBlocked     []string  `json:"custom_blocked"`
	CustomAllowed     []string  `json:"custom_allowed"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AbuseReport struct {
	ID            string    `json:"id"`
	ReporterName  string    `json:"reporter_name"`
	ReporterEmail string    `json:"reporter_email"`
	AbuseType     string    `json:"abuse_type"`
	Subdomain     string    `json:"subdomain"`
	Description   string    `json:"description"`
	Evidence      string    `json:"evidence"`
	Status        string    `json:"status"` // pending, investigating, resolved_blocked, dismissed
	AdminNotes    string    `json:"admin_notes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GrowthStats struct {
	ActiveSubdomains int `json:"active_subdomains"`
	ActiveRecords    int `json:"active_records"`
	TotalQueries     int `json:"total_queries"`
	BlockedThreats   int `json:"blocked_threats"`
	BlockedDomains   int `json:"blocked_domains"`
}

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

	CREATE INDEX IF NOT EXISTS idx_records_subdomain ON records(subdomain);
	CREATE INDEX IF NOT EXISTS idx_records_lookup ON records(subdomain, name, type);
	CREATE INDEX IF NOT EXISTS idx_requests_subdomain ON requests(subdomain);
	CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at);
	CREATE INDEX IF NOT EXISTS idx_abuse_status ON abuse_reports(status);
	CREATE INDEX IF NOT EXISTS idx_abuse_subdomain ON abuse_reports(subdomain);
	`

	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

// User methods
func (d *DB) CreateUser(id, subdomain string) error {
	_, err := d.conn.Exec(
		"INSERT INTO users (id, subdomain, created_at, last_active) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)",
		id, subdomain,
	)
	return err
}

func (d *DB) GetUserBySubdomain(subdomain string) (*UserSession, error) {
	row := d.conn.QueryRow("SELECT id, subdomain, created_at, last_active FROM users WHERE subdomain = ?", subdomain)
	var u UserSession
	if err := row.Scan(&u.ID, &u.Subdomain, &u.CreatedAt, &u.LastActive); err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) TouchUser(subdomain string) {
	_, _ = d.conn.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE subdomain = ?", subdomain)
}

// Record methods
func (d *DB) GetRecords(subdomain string) ([]Record, error) {
	rows, err := d.conn.Query(
		"SELECT id, subdomain, name, type, value, ttl, created_at, updated_at FROM records WHERE subdomain = ? ORDER BY type ASC, name ASC",
		subdomain,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.Subdomain, &r.Name, &r.Type, &r.Value, &r.TTL, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func (d *DB) GetAllActiveRecords() ([]Record, error) {
	rows, err := d.conn.Query(
		"SELECT id, subdomain, name, type, value, ttl, created_at, updated_at FROM records ORDER BY subdomain ASC, name ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.Subdomain, &r.Name, &r.Type, &r.Value, &r.TTL, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func (d *DB) GetRecordsByNameAndType(subdomain, name, recordType string) ([]Record, error) {
	var rows *sql.Rows
	var err error

	if recordType == "ANY" {
		rows, err = d.conn.Query(
			"SELECT id, subdomain, name, type, value, ttl, created_at, updated_at FROM records WHERE subdomain = ? AND (name = ? OR name = '@')",
			subdomain, name,
		)
	} else {
		rows, err = d.conn.Query(
			"SELECT id, subdomain, name, type, value, ttl, created_at, updated_at FROM records WHERE subdomain = ? AND (name = ? OR (name = '@' AND ? = '')) AND (type = ? OR type = 'CNAME')",
			subdomain, name, name, recordType,
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.Subdomain, &r.Name, &r.Type, &r.Value, &r.TTL, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func (d *DB) AddRecord(r *Record) error {
	_, err := d.conn.Exec(
		"INSERT INTO records (id, subdomain, name, type, value, ttl, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)",
		r.ID, r.Subdomain, r.Name, r.Type, r.Value, r.TTL,
	)
	return err
}

func (d *DB) UpdateRecord(r *Record) error {
	_, err := d.conn.Exec(
		"UPDATE records SET name = ?, type = ?, value = ?, ttl = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND subdomain = ?",
		r.Name, r.Type, r.Value, r.TTL, r.ID, r.Subdomain,
	)
	return err
}

func (d *DB) DeleteRecord(subdomain, id string) error {
	_, err := d.conn.Exec("DELETE FROM records WHERE id = ? AND subdomain = ?", id, subdomain)
	return err
}

func (d *DB) DeleteAllRecords(subdomain string) error {
	_, err := d.conn.Exec("DELETE FROM records WHERE subdomain = ?", subdomain)
	return err
}

// Request logging methods
func (d *DB) LogRequest(req *RequestLog, answersJSON string) error {
	_, err := d.conn.Exec(
		"INSERT INTO requests (id, subdomain, qname, qtype, client_ip, rcode, answers, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
		req.ID, req.Subdomain, req.QName, req.QType, req.ClientIP, req.RCode, answersJSON,
	)
	return err
}

func (d *DB) GetRecentRequests(subdomain string, limit int) ([]RequestLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.conn.Query(
		"SELECT id, subdomain, qname, qtype, client_ip, rcode, answers, created_at FROM requests WHERE subdomain = ? ORDER BY created_at DESC LIMIT ?",
		subdomain, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]RequestLog, 0)
	for rows.Next() {
		var req RequestLog
		var answersRaw string
		if err := rows.Scan(&req.ID, &req.Subdomain, &req.QName, &req.QType, &req.ClientIP, &req.RCode, &answersRaw, &req.CreatedAt); err != nil {
			return nil, err
		}
		// Quick parse of JSON array
		req.Answers = parseStringSlice(answersRaw)
		requests = append(requests, req)
	}
	return requests, nil
}

func (d *DB) DeleteRequests(subdomain string) error {
	_, err := d.conn.Exec("DELETE FROM requests WHERE subdomain = ?", subdomain)
	return err
}

// Cleanup old entries (records and requests older than 14 days)
func (d *DB) CleanupOldData() {
	twoWeeksAgo := time.Now().Add(-14 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	res1, err := d.conn.Exec("DELETE FROM requests WHERE created_at < ?", twoWeeksAgo)
	if err == nil {
		rows, _ := res1.RowsAffected()
		if rows > 0 {
			log.Printf("[DB] Cleaned up %d old requests", rows)
		}
	}
	res2, err := d.conn.Exec("DELETE FROM users WHERE last_active < ?", twoWeeksAgo)
	if err == nil {
		rows, _ := res2.RowsAffected()
		if rows > 0 {
			log.Printf("[DB] Cleaned up %d inactive users and their zones", rows)
		}
	}
}

// Parental Control methods
func (d *DB) GetParentalConfig(subdomain string) (*ParentalConfig, error) {
	row := d.conn.QueryRow(`
		SELECT subdomain, enabled, block_adult, block_gambling, block_malware, block_ads, block_social, block_gaming, enforce_safesearch, block_mode, custom_blocked, custom_allowed, updated_at
		FROM parental_configs WHERE subdomain = ?
	`, subdomain)

	var p ParentalConfig
	var customBlockedRaw, customAllowedRaw string
	var enabled, bAdult, bGambling, bMalware, bAds, bSocial, bGaming, safeSearch int

	err := row.Scan(
		&p.Subdomain, &enabled, &bAdult, &bGambling, &bMalware, &bAds, &bSocial, &bGaming,
		&safeSearch, &p.BlockMode, &customBlockedRaw, &customAllowedRaw, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default config
			return &ParentalConfig{
				Subdomain:         subdomain,
				Enabled:           true,
				BlockAdult:        true,
				BlockGambling:     true,
				BlockMalware:      true,
				BlockAds:          true,
				BlockSocial:       false,
				BlockGaming:       false,
				EnforceSafeSearch: true,
				BlockMode:         "0.0.0.0",
				CustomBlocked:     []string{},
				CustomAllowed:     []string{},
				UpdatedAt:         time.Now(),
			}, nil
		}
		return nil, err
	}

	p.Enabled = enabled == 1
	p.BlockAdult = bAdult == 1
	p.BlockGambling = bGambling == 1
	p.BlockMalware = bMalware == 1
	p.BlockAds = bAds == 1
	p.BlockSocial = bSocial == 1
	p.BlockGaming = bGaming == 1
	p.EnforceSafeSearch = safeSearch == 1
	p.CustomBlocked = parseStringSlice(customBlockedRaw)
	p.CustomAllowed = parseStringSlice(customAllowedRaw)

	return &p, nil
}

func (d *DB) SaveParentalConfig(p *ParentalConfig) error {
	blockedJSON, _ := json.Marshal(p.CustomBlocked)
	allowedJSON, _ := json.Marshal(p.CustomAllowed)

	toBoolInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	_, err := d.conn.Exec(`
		INSERT INTO parental_configs (
			subdomain, enabled, block_adult, block_gambling, block_malware, block_ads, block_social, block_gaming, enforce_safesearch, block_mode, custom_blocked, custom_allowed, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(subdomain) DO UPDATE SET
			enabled = excluded.enabled,
			block_adult = excluded.block_adult,
			block_gambling = excluded.block_gambling,
			block_malware = excluded.block_malware,
			block_ads = excluded.block_ads,
			block_social = excluded.block_social,
			block_gaming = excluded.block_gaming,
			enforce_safesearch = excluded.enforce_safesearch,
			block_mode = excluded.block_mode,
			custom_blocked = excluded.custom_blocked,
			custom_allowed = excluded.custom_allowed,
			updated_at = CURRENT_TIMESTAMP
	`,
		p.Subdomain,
		toBoolInt(p.Enabled),
		toBoolInt(p.BlockAdult),
		toBoolInt(p.BlockGambling),
		toBoolInt(p.BlockMalware),
		toBoolInt(p.BlockAds),
		toBoolInt(p.BlockSocial),
		toBoolInt(p.BlockGaming),
		toBoolInt(p.EnforceSafeSearch),
		p.BlockMode,
		string(blockedJSON),
		string(allowedJSON),
	)
	return err
}

func parseStringSlice(raw string) []string {
	if len(raw) <= 2 {
		return []string{}
	}
	var res []string
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return []string{raw}
	}
	return res
}

// Abuse Reports methods
func (d *DB) CreateAbuseReport(report *AbuseReport) error {
	_, err := d.conn.Exec(`
		INSERT INTO abuse_reports (
			id, reporter_name, reporter_email, abuse_type, subdomain, description, evidence, status, admin_notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`,
		report.ID,
		report.ReporterName,
		report.ReporterEmail,
		report.AbuseType,
		report.Subdomain,
		report.Description,
		report.Evidence,
		report.Status,
		report.AdminNotes,
	)
	return err
}

func (d *DB) GetAbuseReports(statusFilter string) ([]*AbuseReport, error) {
	var rows *sql.Rows
	var err error

	if statusFilter != "" && statusFilter != "all" {
		rows, err = d.conn.Query(`
			SELECT id, reporter_name, reporter_email, abuse_type, subdomain, description, evidence, status, admin_notes, created_at, updated_at
			FROM abuse_reports
			WHERE status = ?
			ORDER BY created_at DESC
		`, statusFilter)
	} else {
		rows, err = d.conn.Query(`
			SELECT id, reporter_name, reporter_email, abuse_type, subdomain, description, evidence, status, admin_notes, created_at, updated_at
			FROM abuse_reports
			ORDER BY created_at DESC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*AbuseReport
	for rows.Next() {
		var r AbuseReport
		if err := rows.Scan(
			&r.ID, &r.ReporterName, &r.ReporterEmail, &r.AbuseType, &r.Subdomain,
			&r.Description, &r.Evidence, &r.Status, &r.AdminNotes, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reports = append(reports, &r)
	}
	return reports, nil
}

func (d *DB) GetAbuseReportByID(id string) (*AbuseReport, error) {
	var r AbuseReport
	err := d.conn.QueryRow(`
		SELECT id, reporter_name, reporter_email, abuse_type, subdomain, description, evidence, status, admin_notes, created_at, updated_at
		FROM abuse_reports
		WHERE id = ?
	`, id).Scan(
		&r.ID, &r.ReporterName, &r.ReporterEmail, &r.AbuseType, &r.Subdomain,
		&r.Description, &r.Evidence, &r.Status, &r.AdminNotes, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *DB) UpdateAbuseReport(id, status, notes string) error {
	_, err := d.conn.Exec(`
		UPDATE abuse_reports
		SET status = ?, admin_notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, notes, id)
	return err
}

func (d *DB) DeleteAbuseReport(id string) error {
	_, err := d.conn.Exec("DELETE FROM abuse_reports WHERE id = ?", id)
	return err
}

// Blocked Subdomain methods
func (d *DB) BlockSubdomain(subdomain, reason string) error {
	_, err := d.conn.Exec(`
		INSERT OR REPLACE INTO blocked_subdomains (subdomain, reason, blocked_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, subdomain, reason)
	return err
}

func (d *DB) UnblockSubdomain(subdomain string) error {
	_, err := d.conn.Exec("DELETE FROM blocked_subdomains WHERE subdomain = ?", subdomain)
	return err
}

func (d *DB) IsSubdomainBlocked(subdomain string) (bool, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM blocked_subdomains WHERE subdomain = ?", subdomain).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) GetBlockedSubdomains() ([]map[string]any, error) {
	rows, err := d.conn.Query("SELECT subdomain, reason, blocked_at FROM blocked_subdomains ORDER BY blocked_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		var sub, reason string
		var blockedAt time.Time
		if err := rows.Scan(&sub, &reason, &blockedAt); err != nil {
			return nil, err
		}
		list = append(list, map[string]any{
			"subdomain":  sub,
			"reason":     reason,
			"blocked_at": blockedAt,
		})
	}
	return list, nil
}

// Growth Statistics
func (d *DB) GetGrowthStats() (*GrowthStats, error) {
	stats := &GrowthStats{}

	_ = d.conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.ActiveSubdomains)
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM records").Scan(&stats.ActiveRecords)
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM requests").Scan(&stats.TotalQueries)
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM blocked_subdomains").Scan(&stats.BlockedDomains)

	var blockedAbuseReports int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM abuse_reports WHERE status = 'resolved_blocked'").Scan(&blockedAbuseReports)
	stats.BlockedThreats = stats.BlockedDomains + blockedAbuseReports

	return stats, nil
}
