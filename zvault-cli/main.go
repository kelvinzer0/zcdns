package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const version = "0.1.0"
const backendURL = "https://zcdns.id" // Wait, I should make this configurable, but let's hardcode for now, or read from env.
var apiBase = "https://zcdns.id/api"

func init() {
	if val := os.Getenv("ZVAULT_API"); val != "" {
		apiBase = val
	}
}

func printHelp() {
	fmt.Printf("zvault v%s - Git-style version control for secrets\n\n", version)
	fmt.Println("Usage: zvault <command> [arguments]")
	fmt.Println("\nAuthentication:")
	fmt.Println("  login                 Authenticate CLI with your ZCDNS account")
	fmt.Println("\nRepository & Workflow Commands:")
	fmt.Println("  status                Show working tree status & modified secrets")
	fmt.Println("  set <key=value>       Add or update a secret value")
	fmt.Println("  rm <key>              Remove a secret from local working tree")
	fmt.Println("  commit [-m msg]       Record changes to the local vault repository")
	fmt.Println("  log                   Show commit logs from the remote vault")
	fmt.Println("  show                  Show all secrets in the current working directory")
	fmt.Println("  diff                  Show changes between the working tree and the latest commit")
	fmt.Println("  push                  Push local commits to remote vault")
	fmt.Println("  pull                  Fetch and integrate remote changes")
	fmt.Println("  branch [-d name]      List all branches, or delete a branch with -d")
	fmt.Println("  switch <branch>       Switch to a different branch")
	fmt.Println("\nSecret Lifecycle Commands:")
	fmt.Println("  revoke <commit>       Revert the vault back to a specific commit hash")
	fmt.Println("  destroy               Destroy and remove the local vault completely")
	fmt.Println("\nOther Commands:")
	fmt.Println("  version, -v           Show version information")
	fmt.Println("  help, -h              Show this help menu")
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]
	switch command {
	case "version", "-v", "--version":
		fmt.Printf("zvault version %s\n", version)
	case "help", "-h", "--help":
		printHelp()
	case "login":
		cmdLogin()
	case "set":
		cmdSet(os.Args[2:])
	case "rm":
		cmdRm(os.Args[2:])
	case "commit":
		cmdCommit(os.Args[2:])
	case "push":
		cmdPush()
	case "pull":
		cmdPull()
	case "status":
		cmdStatus()
	case "diff":
		cmdDiff()
	case "log":
		cmdLog()
	case "show":
		cmdShow()
	case "branch":
		cmdBranch(os.Args[2:])
	case "switch":
		cmdSwitch(os.Args[2:])
	case "revoke":
		cmdRevoke(os.Args[2:])
	case "destroy":
		cmdDestroy()
	case "rotate":
		fmt.Printf("zvault: '%s' is in active preview mode. Implementation coming soon!\n", command)
	default:
		fmt.Printf("zvault: '%s' is not a valid command.\n", command)
		os.Exit(1)
	}
}

// ... more code to be added ...

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

type Config struct {
	Token     string `json:"token"`
	Subdomain string `json:"subdomain"`
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zvault", "config.json")
}

func loadConfig() (*Config, error) {
	b, err := os.ReadFile(getConfigPath())
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func cmdLogin() {
	resp, err := http.Post(apiBase+"/vault/auth/device", "application/json", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	var d DeviceAuthResp
	json.NewDecoder(resp.Body).Decode(&d)

	fmt.Printf("Please open: %s\n", d.VerificationUriComplete)
	fmt.Printf("Or open %s and enter code: %s\n", d.VerificationUri, d.UserCode)

	for {
		time.Sleep(2 * time.Second)
		reqBody := fmt.Sprintf(`{"device_code":"%s"}`, d.DeviceCode)
		pResp, err := http.Post(apiBase+"/vault/auth/poll", "application/json", strings.NewReader(reqBody))
		if err != nil {
			continue
		}
		var p PollResp
		json.NewDecoder(pResp.Body).Decode(&p)
		pResp.Body.Close()

		if p.Status == "approved" {
			cfgPath := getConfigPath()
			os.MkdirAll(filepath.Dir(cfgPath), 0755)
			b, _ := json.Marshal(Config{Token: p.Token, Subdomain: p.Subdomain})
			os.WriteFile(cfgPath, b, 0644)
			fmt.Printf("Successfully logged in as %s\n", p.Subdomain)
			return
		} else if p.Status == "denied" || p.Status == "expired" {
			fmt.Println("Auth failed:", p.Status)
			return
		}
	}
}

func getProjectDir() string {
	return ".zvault"
}

func initProject() {
	os.MkdirAll(".zvault/commits", 0755)
	if _, err := os.Stat(".zvault/head.json"); os.IsNotExist(err) {
		os.WriteFile(".zvault/head.json", []byte("{}"), 0644)
	}
	if _, err := os.Stat(".zvault/working.json"); os.IsNotExist(err) {
		os.WriteFile(".zvault/working.json", []byte("{}"), 0644)
	}
}

func cmdSet(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: zvault set <key=value>")
		return
	}
	parts := strings.SplitN(args[0], "=", 2)
	if len(parts) != 2 {
		fmt.Println("Usage: zvault set <key=value>")
		return
	}
	initProject()
	b, _ := os.ReadFile(".zvault/working.json")
	var working map[string]string
	json.Unmarshal(b, &working)
	if working == nil {
		working = make(map[string]string)
	}
	working[parts[0]] = parts[1]
	wb, _ := json.MarshalIndent(working, "", "  ")
	os.WriteFile(".zvault/working.json", wb, 0644)
	fmt.Printf("Set %s\n", parts[0])
}

type VaultCommit struct {
	ID         string    `json:"id"`
	RepoID     string    `json:"repo_id"`
	Hash       string    `json:"hash"`
	ParentHash string    `json:"parent_hash"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Author     string    `json:"author"`
}

type VaultKVPair struct {
	ID             string `json:"id"`
	CommitHash     string `json:"commit_hash"`
	KeyName        string `json:"key_name"`
	EncryptedValue string `json:"encrypted_value"`
}

func generateHash() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)[:8]
}

func cmdCommit(args []string) {
	msg := "update"
	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			msg = args[i+1]
		}
	}
	initProject()
	b, _ := os.ReadFile(".zvault/working.json")
	var working map[string]string
	json.Unmarshal(b, &working)

	hb, _ := os.ReadFile(".zvault/head.json")
	var head map[string]string
	json.Unmarshal(hb, &head)

	// In a real app we'd calc diff, here we just save working as the new state
	hash := generateHash()

	// Parent hash
	var parentHash string
	ph, _ := os.ReadFile(".zvault/head_commit")
	parentHash = strings.TrimSpace(string(ph))

	c := VaultCommit{
		ID:         generateHash() + generateHash(), // dummy uuid
		Hash:       hash,
		ParentHash: parentHash,
		Message:    msg,
		Timestamp:  time.Now(),
		Author:     "local",
	}

	var pairs []VaultKVPair
	for k, v := range working {
		pairs = append(pairs, VaultKVPair{
			ID:             generateHash() + generateHash(),
			CommitHash:     hash,
			KeyName:        k,
			EncryptedValue: v, // dummy encryption
		})
	}

	cData := map[string]any{"commit": c, "pairs": pairs}
	cb, _ := json.Marshal(cData)
	os.WriteFile(filepath.Join(".zvault/commits", hash+".json"), cb, 0644)
	os.WriteFile(".zvault/head_commit", []byte(hash), 0644)
	// head.json becomes working.json
	os.WriteFile(".zvault/head.json", b, 0644)

	fmt.Printf("Committed %s\n", hash)
}

func cmdPush() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	// In a real implementation we would fetch init to get repo ID
	req, _ := http.NewRequest("POST", apiBase+"/vault/init", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	var initResp map[string]any
	json.NewDecoder(resp.Body).Decode(&initResp)
	resp.Body.Close()

	repoID, _ := initResp["id"].(string)
	if repoID == "" {
		fmt.Println("Failed to init remote vault")
		return
	}

	// Gather commits
	files, _ := os.ReadDir(".zvault/commits")
	var commits []VaultCommit
	var allPairs []VaultKVPair
	for _, f := range files {
		b, _ := os.ReadFile(filepath.Join(".zvault/commits", f.Name()))
		var cData struct {
			Commit VaultCommit   `json:"commit"`
			Pairs  []VaultKVPair `json:"pairs"`
		}
		json.Unmarshal(b, &cData)
		cData.Commit.RepoID = repoID
		commits = append(commits, cData.Commit)
		allPairs = append(allPairs, cData.Pairs...)
	}

	ph, _ := os.ReadFile(".zvault/head_commit")
	headHash := strings.TrimSpace(string(ph))

	payload := map[string]any{
		"repo_id":          repoID,
		"commits":          commits,
		"kv_pairs":         allPairs,
		"head_commit_hash": headHash,
	}
	pb, _ := json.Marshal(payload)

	req, _ = http.NewRequest("POST", apiBase+"/vault/sync", bytes.NewReader(pb))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req)
	if err != nil {
		fmt.Println("Push error:", err)
		return
	}
	defer resp2.Body.Close()
	b2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode == 200 {
		fmt.Println("Push successful")
	} else {
		fmt.Println("Push failed:", string(b2))
	}
}

func cmdPull() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	repoID, err := getRepoID(cfg)
	if err != nil || repoID == "" {
		fmt.Println("Failed to get remote repository ID")
		return
	}

	// 1. Get latest commits
	req, _ := http.NewRequest("GET", apiBase+"/vault/commits?repo_id="+repoID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Subdomain", cfg.Subdomain)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("Failed to fetch remote commits")
		return
	}
	defer resp.Body.Close()
	var commits []VaultCommit
	json.NewDecoder(resp.Body).Decode(&commits)
	
	if len(commits) == 0 {
		fmt.Println("Remote repository is empty.")
		return
	}
	latestCommit := commits[0].Hash

	// 2. Fetch KV pairs for the latest commit
	kvReq, _ := http.NewRequest("GET", apiBase+"/vault/kv?commit="+latestCommit, nil)
	kvReq.Header.Set("Authorization", "Bearer "+cfg.Token)
	kvReq.Header.Set("X-Subdomain", cfg.Subdomain)
	kvResp, err := client.Do(kvReq)
	if err != nil || kvResp.StatusCode != 200 {
		fmt.Println("Failed to fetch remote KV pairs")
		return
	}
	defer kvResp.Body.Close()
	var pairs []VaultKVPair
	json.NewDecoder(kvResp.Body).Decode(&pairs)

	// 3. Save to local working tree
	initProject()
	working := make(map[string]string)
	for _, p := range pairs {
		working[p.KeyName] = p.EncryptedValue
	}
	
	wb, _ := json.MarshalIndent(working, "", "  ")
	os.WriteFile(".zvault/working.json", wb, 0644)
	os.WriteFile(".zvault/head.json", wb, 0644)
	os.WriteFile(".zvault/head_commit", []byte(latestCommit), 0644)

	fmt.Printf("Successfully pulled latest changes (commit %s).\n", latestCommit)
}
