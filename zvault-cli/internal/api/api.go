package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"zvault-cli/internal/config"
	"zvault-cli/internal/workspace"
)

var BaseURL = "https://zcdns.id/api"

func init() {
	if val := os.Getenv("ZVAULT_API"); val != "" {
		BaseURL = val
	}
}

type DeviceAuthResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationUri         string `json:"verification_uri"`
	VerificationUriComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type PollResp struct {
	Status    string `json:"status"`
	Token     string `json:"token"`
	Subdomain string `json:"subdomain"`
	Error     string `json:"error"`
}

func GetRepoID(cfg *config.Config) (string, error) {
	req, _ := http.NewRequest("POST", BaseURL+"/vault/init", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var initResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return "", err
	}

	repoID, _ := initResp["id"].(string)
	return repoID, nil
}

func FetchCommits(cfg *config.Config, repoID string) ([]workspace.VaultCommit, error) {
	req, _ := http.NewRequest("GET", BaseURL+"/vault/commits?repo_id="+repoID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status %d", resp.StatusCode)
	}

	var commits []workspace.VaultCommit
	err = json.NewDecoder(resp.Body).Decode(&commits)
	return commits, err
}

func FetchKVPairs(cfg *config.Config, commitHash string) ([]workspace.VaultKVPair, error) {
	req, _ := http.NewRequest("GET", BaseURL+"/vault/kv?commit="+commitHash, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status %d", resp.StatusCode)
	}
	var pairs []workspace.VaultKVPair
	err = json.NewDecoder(resp.Body).Decode(&pairs)
	return pairs, err
}

func Sync(cfg *config.Config, repoID string, commits []workspace.VaultCommit, allPairs []workspace.VaultKVPair, headHash string) error {
	payload := map[string]any{
		"repo_id":          repoID,
		"commits":          commits,
		"kv_pairs":         allPairs,
		"head_commit_hash": headHash,
	}
	pb, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", BaseURL+"/vault/sync", bytes.NewReader(pb))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sync failed: %s", string(b))
	}
	return nil
}

func GetBranches(cfg *config.Config, repoID string) ([]map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", BaseURL+"/vault/branches?repo_id="+repoID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status")
	}
	var data []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

func DeleteBranch(cfg *config.Config, repoID string, branchName string) error {
	req, _ := http.NewRequest("DELETE", BaseURL+"/vault/branches?repo_id="+repoID+"&name="+branchName, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s", string(b))
	}
	return nil
}

func Revoke(cfg *config.Config, commitHash string) error {
	reqBody := fmt.Sprintf(`{"commit_hash":"%s"}`, commitHash)
	req, _ := http.NewRequest("POST", BaseURL+"/vault/revert", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke failed: %s", string(b))
	}
	return nil
}
