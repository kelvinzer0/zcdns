package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (d *DB) InitVault(subdomain string) (*VaultRepo, error) {
	repo := &VaultRepo{
		ID:        uuid.New().String(),
		Subdomain: subdomain,
		CreatedAt: time.Now(),
	}
	_, err := d.conn.Exec("INSERT INTO vault_repos (id, subdomain) VALUES (?, ?)", repo.ID, repo.Subdomain)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
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
