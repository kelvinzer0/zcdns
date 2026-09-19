package cmd

import (
	"fmt"
	"time"

	"zvault-cli/internal/api"
	"zvault-cli/internal/config"
	"zvault-cli/internal/workspace"
)

func cmdLog() {
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
		fmt.Println("Failed to fetch log:", err)
		return
	}

	for _, c := range commits {
		fmt.Printf("commit %s\n", c.Hash)
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %s\n\n", c.Timestamp.Format(time.RFC1123Z))
		fmt.Printf("    %s\n\n", c.Message)
	}
}

func cmdShow() {
	working, _ := workspace.LoadWorking()
	
	if len(working) == 0 {
		fmt.Println("No secrets in working directory.")
		return
	}
	
	for k, v := range working {
		fmt.Printf("%s=%s\n", k, v)
	}
}
