package db

import "time"

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
