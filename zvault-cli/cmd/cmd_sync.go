package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"zvault-cli/internal/api"
	"zvault-cli/internal/config"
	"zvault-cli/internal/workspace"
)

func cmdPush() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	repoID, err := api.GetRepoID(cfg)
	if err != nil || repoID == "" {
		fmt.Println("Failed to init remote vault")
		return
	}

	files, _ := os.ReadDir(".zvault/commits")
	var commits []workspace.VaultCommit
	var allPairs []workspace.VaultKVPair
	for _, f := range files {
		b, _ := os.ReadFile(filepath.Join(".zvault/commits", f.Name()))
		var cData struct {
			Commit workspace.VaultCommit   `json:"commit"`
			Pairs  []workspace.VaultKVPair `json:"pairs"`
		}
		json.Unmarshal(b, &cData)
		cData.Commit.RepoID = repoID
		commits = append(commits, cData.Commit)
		allPairs = append(allPairs, cData.Pairs...)
	}

	headHash := workspace.LoadHeadCommit()

	err = api.Sync(cfg, repoID, commits, allPairs, headHash)
	if err != nil {
		fmt.Println("Push failed:", err)
	} else {
		fmt.Println("Push successful")
	}
}

func cmdPull() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	repoID, err := api.GetRepoID(cfg)
	if err != nil || repoID == "" {
		fmt.Println("Failed to get remote repository ID")
		return
	}

	commits, err := api.FetchCommits(cfg, repoID)
	if err != nil {
		fmt.Println("Failed to fetch remote commits:", err)
		return
	}
	if len(commits) == 0 {
		fmt.Println("Remote repository is empty.")
		return
	}
	latestCommit := commits[0].Hash

	pairs, err := api.FetchKVPairs(cfg, latestCommit)
	if err != nil {
		fmt.Println("Failed to fetch remote KV pairs:", err)
		return
	}

	workspace.InitProject()
	working := make(map[string]string)
	for _, p := range pairs {
		working[p.KeyName] = p.EncryptedValue
	}

	workspace.SaveWorking(working)
	workspace.SaveHead(working)
	workspace.SaveHeadCommit(latestCommit)

	fmt.Printf("Successfully pulled latest changes (commit %s).\n", latestCommit)
}
