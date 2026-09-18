package db

import (
	"fmt"
)

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

// DeleteSubdomain completely removes a subdomain, its user, records, requests, and parental configurations.
func (d *DB) DeleteSubdomain(subdomain string) error {
	_, _ = d.conn.Exec("DELETE FROM records WHERE subdomain = ?", subdomain)
	_, _ = d.conn.Exec("DELETE FROM requests WHERE subdomain = ?", subdomain)
	_, _ = d.conn.Exec("DELETE FROM parental_configs WHERE subdomain = ?", subdomain)
	_, err := d.conn.Exec("DELETE FROM users WHERE subdomain = ?", subdomain)
	return err
}

func (d *DB) RenewUser(subdomain string) (*UserSession, error) {
	res, err := d.conn.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE subdomain = ?", subdomain)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("subdomain not found")
	}
	return d.GetUserBySubdomain(subdomain)
}
