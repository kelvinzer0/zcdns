package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func getRepoID(cfg *Config) (string, error) {
	req, _ := http.NewRequest("POST", apiBase+"/vault/init", nil)
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
