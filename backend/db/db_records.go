package db

import (
	"database/sql"
)

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

func (d *DB) DeleteRecordByID(id string) error {
	_, err := d.conn.Exec("DELETE FROM records WHERE id = ?", id)
	return err
}

func (d *DB) GetAllTXTRecords() ([]Record, error) {
	rows, err := d.conn.Query(
		"SELECT id, subdomain, name, type, value, ttl, created_at, updated_at FROM records WHERE type = 'TXT' ORDER BY subdomain ASC, name ASC",
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

func (d *DB) DeleteAllRecords(subdomain string) error {
	_, err := d.conn.Exec("DELETE FROM records WHERE subdomain = ?", subdomain)
	return err
}

const MaxTotalRecords = 9999

func (d *DB) GetTotalRecordCount() (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM records").Scan(&count)
	return count, err
}
