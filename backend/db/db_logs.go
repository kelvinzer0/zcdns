package db

import (
	"encoding/json"
	"log"
	"time"
)

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

// Cleanup old entries (requests older than 7 days, and inactive users/subdomains older than 6 months / 180 days)
func (d *DB) CleanupOldData() []string {
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	res1, err := d.conn.Exec("DELETE FROM requests WHERE created_at < ?", sevenDaysAgo)
	if err == nil {
		rows, _ := res1.RowsAffected()
		if rows > 0 {
			log.Printf("[DB] Cleaned up %d old request logs (>7 days)", rows)
		}
	}

	sixMonthsAgo := time.Now().Add(-180 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	rows, err := d.conn.Query("SELECT subdomain FROM users WHERE last_active < ?", sixMonthsAgo)
	var purgedSubs []string
	if err == nil {
		for rows.Next() {
			var sub string
			if err := rows.Scan(&sub); err == nil {
				purgedSubs = append(purgedSubs, sub)
			}
		}
		rows.Close()

		for _, sub := range purgedSubs {
			_, _ = d.conn.Exec("DELETE FROM records WHERE subdomain = ?", sub)
			_, _ = d.conn.Exec("DELETE FROM requests WHERE subdomain = ?", sub)
			_, _ = d.conn.Exec("DELETE FROM parental_configs WHERE subdomain = ?", sub)
			_, _ = d.conn.Exec("DELETE FROM users WHERE subdomain = ?", sub)
			log.Printf("[DB-CLEANUP] Purged expired subdomain '%s' (no renewal/activity for >6 months)", sub)
		}
	}
	return purgedSubs
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
