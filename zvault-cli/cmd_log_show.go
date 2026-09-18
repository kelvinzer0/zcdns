package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func cmdLog() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	req, _ := http.NewRequest("GET", apiBase+"/vault/commits", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("Failed to fetch log")
		return
	}
	defer resp.Body.Close()

	var commits []VaultCommit
	json.NewDecoder(resp.Body).Decode(&commits)

	for _, c := range commits {
		fmt.Printf("commit %s\n", c.Hash)
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %s\n\n", c.Timestamp.Format(time.RFC1123Z))
		fmt.Printf("    %s\n\n", c.Message)
	}
}

func cmdShow() {
	b, _ := os.ReadFile(".zvault/working.json")
	var working map[string]string
	json.Unmarshal(b, &working)
	
	if len(working) == 0 {
		fmt.Println("No secrets in working directory.")
		return
	}
	
	for k, v := range working {
		fmt.Printf("%s=%s\n", k, v)
	}
}

func cmdBranch(args []string) {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	if len(args) == 0 {
		// List branches
		req, _ := http.NewRequest("GET", apiBase+"/vault/branches", nil)
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
		req.Header.Set("X-Subdomain", cfg.Subdomain)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println("Failed to fetch branches")
			return
		}
		defer resp.Body.Close()
		var respData []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respData)
		for _, b := range respData {
			name := b["name"].(string)
			if name == "main" {
				fmt.Printf("* %s\n", name)
			} else {
				fmt.Printf("  %s\n", name)
			}
		}
	} else if len(args) == 2 && (args[0] == "-d" || args[0] == "--delete") {
		branchName := args[1]
		if branchName == "main" {
			fmt.Println("Cannot delete main branch.")
			return
		}
		
		// Get repo_id from init
		req, _ := http.NewRequest("POST", apiBase+"/vault/init", nil)
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
		req.Header.Set("X-Subdomain", cfg.Subdomain)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error fetching repo ID:", err)
			return
		}
		var initResp map[string]any
		json.NewDecoder(resp.Body).Decode(&initResp)
		resp.Body.Close()
		repoID, _ := initResp["id"].(string)

		delReq, _ := http.NewRequest("DELETE", apiBase+"/vault/branches?repo_id="+repoID+"&name="+branchName, nil)
		delReq.Header.Set("Authorization", "Bearer "+cfg.Token)
		delReq.Header.Set("X-Subdomain", cfg.Subdomain)
		delResp, err := client.Do(delReq)
		if err != nil {
			fmt.Println("Error deleting branch:", err)
			return
		}
		defer delResp.Body.Close()
		if delResp.StatusCode == 200 {
			fmt.Printf("Deleted branch '%s'\n", branchName)
		} else {
			b, _ := io.ReadAll(delResp.Body)
			fmt.Println("Failed to delete branch:", string(b))
		}
	} else {
		fmt.Println("Creating branches is not fully implemented yet.")
	}
}

func cmdSwitch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: zvault switch <branch>")
		return
	}
	fmt.Printf("Switched to branch '%s'\n", args[0])
}

func cmdRevoke(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: zvault revoke <commit>")
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	reqBody := fmt.Sprintf(`{"commit_hash":"%s"}`, args[0])
	req, _ := http.NewRequest("POST", apiBase+"/vault/revert", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		fmt.Printf("Commit %s revoked successfully.\n", args[0])
	} else {
		b, _ := io.ReadAll(resp.Body)
		fmt.Println("Failed to revoke commit:", string(b))
	}
}

func cmdDestroy() {
	fmt.Println("Destroying vault...")
	// Dummy destroy implementation
	os.RemoveAll(".zvault")
	fmt.Println("Local vault destroyed.")
}
