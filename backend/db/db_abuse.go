package db

import (
	"database/sql"
	"time"
)

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
