package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (d *DB) InitVault(subdomain string) (*VaultRepo, error) {
	var existingID string
	err := d.conn.QueryRow("SELECT id FROM vault_repos WHERE subdomain = ?", subdomain).Scan(&existingID)
	if err == nil {
		return &VaultRepo{ID: existingID, Subdomain: subdomain}, nil
	}

	repo := &VaultRepo{
		ID:        uuid.New().String(),
		Subdomain: subdomain,
		CreatedAt: time.Now(),
	}
	_, err = d.conn.Exec("INSERT INTO vault_repos (id, subdomain) VALUES (?, ?)", repo.ID, repo.Subdomain)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}

	branchID := uuid.New().String()
	commitHash := uuid.New().String()[:8]

	_, err = d.conn.Exec("INSERT INTO vault_commits (id, repo_id, hash, message, timestamp, author) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New().String(), repo.ID, commitHash, "initial commit", time.Now(), "system")
	if err != nil {
		return nil, err
	}

	_, err = d.conn.Exec("INSERT INTO vault_branches (id, repo_id, name, head_commit, updated_at) VALUES (?, ?, ?, ?, ?)",
		branchID, repo.ID, "main", commitHash, time.Now())
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func (d *DB) GetBranches(repoID string) ([]VaultBranch, error) {
	rows, err := d.conn.Query("SELECT id, repo_id, name, head_commit, updated_at FROM vault_branches WHERE repo_id = ?", repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var branches []VaultBranch
	for rows.Next() {
		var b VaultBranch
		var headCommit *string
		if err := rows.Scan(&b.ID, &b.RepoID, &b.Name, &headCommit, &b.UpdatedAt); err != nil {
			return nil, err
		}
		if headCommit != nil {
			b.HeadCommit = *headCommit
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func (d *DB) GetCommits(repoID string) ([]VaultCommit, error) {
	rows, err := d.conn.Query("SELECT id, repo_id, hash, parent_hash, message, timestamp, author FROM vault_commits WHERE repo_id = ?", repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var commits []VaultCommit
	for rows.Next() {
		var c VaultCommit
		var parentHash *string
		if err := rows.Scan(&c.ID, &c.RepoID, &c.Hash, &parentHash, &c.Message, &c.Timestamp, &c.Author); err != nil {
			return nil, err
		}
		if parentHash != nil {
			c.ParentHash = *parentHash
		}
		commits = append(commits, c)
	}
	return commits, nil
}

func (d *DB) CreateVaultAuthRequest(req *VaultAuthRequest) error {
	_, err := d.conn.Exec(`
		INSERT INTO vault_auth_requests (device_code, user_code, status, expires_at)
		VALUES (?, ?, ?, ?)
	`, req.DeviceCode, req.UserCode, req.Status, req.ExpiresAt)
	return err
}

func (d *DB) GetVaultAuthByDeviceCode(deviceCode string) (*VaultAuthRequest, error) {
	var req VaultAuthRequest
	var sub, tok *string
	err := d.conn.QueryRow(`
		SELECT device_code, user_code, subdomain, token, status, expires_at, created_at
		FROM vault_auth_requests WHERE device_code = ?
	`, deviceCode).Scan(&req.DeviceCode, &req.UserCode, &sub, &tok, &req.Status, &req.ExpiresAt, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	if sub != nil {
		req.Subdomain = *sub
	}
	if tok != nil {
		req.Token = *tok
	}
	return &req, nil
}

func (d *DB) GetVaultAuthByUserCode(userCode string) (*VaultAuthRequest, error) {
	var req VaultAuthRequest
	var sub, tok *string
	err := d.conn.QueryRow(`
		SELECT device_code, user_code, subdomain, token, status, expires_at, created_at
		FROM vault_auth_requests WHERE user_code = ?
	`, userCode).Scan(&req.DeviceCode, &req.UserCode, &sub, &tok, &req.Status, &req.ExpiresAt, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	if sub != nil {
		req.Subdomain = *sub
	}
	if tok != nil {
		req.Token = *tok
	}
	return &req, nil
}

func (d *DB) ApproveVaultAuth(userCode, subdomain, token string) error {
	_, err := d.conn.Exec(`
		UPDATE vault_auth_requests
		SET status = 'approved', subdomain = ?, token = ?
		WHERE user_code = ? AND status = 'pending'
	`, subdomain, token, userCode)
	return err
}

func (d *DB) DenyVaultAuth(userCode string) error {
	_, err := d.conn.Exec(`
		UPDATE vault_auth_requests
		SET status = 'denied'
		WHERE user_code = ? AND status = 'pending'
	`, userCode)
	return err
}

func (d *DB) GetVaultKVPairs(commitHash string) ([]VaultKVPair, error) {
	rows, err := d.conn.Query("SELECT id, commit_hash, key_name, encrypted_value FROM vault_kv_pairs WHERE commit_hash = ?", commitHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pairs []VaultKVPair
	for rows.Next() {
		var p VaultKVPair
		if err := rows.Scan(&p.ID, &p.CommitHash, &p.KeyName, &p.EncryptedValue); err != nil {
			return nil, err
		}
		pairs = append(pairs, p)
	}
	return pairs, nil
}

func (d *DB) RevertToCommit(repoID, targetCommitHash string) error {
	var targetTimestamp time.Time
	err := d.conn.QueryRow("SELECT timestamp FROM vault_commits WHERE hash = ?", targetCommitHash).Scan(&targetTimestamp)
	if err != nil {
		return err
	}

	rows, err := d.conn.Query("SELECT hash FROM vault_commits WHERE repo_id = ? AND timestamp > ?", repoID, targetTimestamp)
	if err != nil {
		return err
	}
	var commitsToDelete []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return err
		}
		commitsToDelete = append(commitsToDelete, h)
	}
	rows.Close()

	for _, h := range commitsToDelete {
		_, err = d.conn.Exec("DELETE FROM vault_kv_pairs WHERE commit_hash = ?", h)
		if err != nil {
			return err
		}
		_, err = d.conn.Exec("DELETE FROM vault_commits WHERE hash = ?", h)
		if err != nil {
			return err
		}
	}

	_, err = d.conn.Exec("UPDATE vault_branches SET head_commit = ? WHERE repo_id = ?", targetCommitHash, repoID)
	return err
}

func (d *DB) DeleteCommit(repoID, commitHash string) error {
	_, err := d.conn.Exec("DELETE FROM vault_kv_pairs WHERE commit_hash = ?", commitHash)
	if err != nil {
		return err
	}

	var parentHash string
	err = d.conn.QueryRow("SELECT COALESCE(parent_hash, '') FROM vault_commits WHERE hash = ?", commitHash).Scan(&parentHash)
	if err != nil {
		return err
	}

	_, err = d.conn.Exec("DELETE FROM vault_commits WHERE hash = ?", commitHash)
	if err != nil {
		return err
	}

	if parentHash != "" {
		_, err = d.conn.Exec("UPDATE vault_branches SET head_commit = ? WHERE repo_id = ? AND head_commit = ?", parentHash, repoID, commitHash)
	} else {
		_, err = d.conn.Exec("UPDATE vault_branches SET head_commit = NULL WHERE repo_id = ? AND head_commit = ?", repoID, commitHash)
	}
	return err
}

func (d *DB) SyncVault(repoID string, commits []VaultCommit, kvPairs []VaultKVPair, headCommitHash string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, c := range commits {
		_, err = tx.Exec("INSERT OR IGNORE INTO vault_commits (id, repo_id, hash, parent_hash, message, timestamp, author) VALUES (?, ?, ?, ?, ?, ?, ?)",
			c.ID, c.RepoID, c.Hash, c.ParentHash, c.Message, c.Timestamp, c.Author)
		if err != nil {
			return err
		}
	}

	for _, p := range kvPairs {
		_, err = tx.Exec("INSERT OR IGNORE INTO vault_kv_pairs (id, commit_hash, key_name, encrypted_value) VALUES (?, ?, ?, ?)",
			p.ID, p.CommitHash, p.KeyName, p.EncryptedValue)
		if err != nil {
			return err
		}
	}

	if headCommitHash != "" {
		_, err = tx.Exec("UPDATE vault_branches SET head_commit = ?, updated_at = ? WHERE repo_id = ? AND name = 'main'",
			headCommitHash, time.Now(), repoID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) VerifyRepoOwnership(repoID, subdomain string) bool {
	var owner string
	err := d.conn.QueryRow("SELECT subdomain FROM vault_repos WHERE id = ?", repoID).Scan(&owner)
	if err != nil {
		return false
	}
	return owner == subdomain
}

func (d *DB) VerifyCommitOwnership(commitHash, subdomain string) bool {
	var owner string
	err := d.conn.QueryRow("SELECT r.subdomain FROM vault_repos r JOIN vault_commits c ON r.id = c.repo_id WHERE c.hash = ?", commitHash).Scan(&owner)
	if err != nil {
		return false
	}
	return owner == subdomain
}

func (d *DB) DeleteBranch(repoID, branchName string) error {
	_, err := d.conn.Exec(`DELETE FROM vault_branches WHERE repo_id = ? AND name = ?`, repoID, branchName)
	return err
}
