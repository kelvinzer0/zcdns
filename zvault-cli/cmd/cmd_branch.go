package cmd

import (
	"fmt"

	"zvault-cli/internal/api"
	"zvault-cli/internal/config"
)

func cmdBranch(args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	if len(args) == 0 {
		repoID, err := api.GetRepoID(cfg)
		if err != nil || repoID == "" {
			fmt.Println("Failed to get remote repository ID")
			return
		}

		branches, err := api.GetBranches(cfg, repoID)
		if err != nil {
			fmt.Println("Failed to fetch branches:", err)
			return
		}
		for _, b := range branches {
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
		
		repoID, err := api.GetRepoID(cfg)
		if err != nil {
			fmt.Println("Error fetching repo ID:", err)
			return
		}
		
		err = api.DeleteBranch(cfg, repoID, branchName)
		if err != nil {
			fmt.Println("Error deleting branch:", err)
		} else {
			fmt.Printf("Deleted branch '%s'\n", branchName)
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
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("Please run 'zvault login' first.")
		return
	}

	err = api.Revoke(cfg, args[0])
	if err != nil {
		fmt.Println("Failed to revoke commit:", err)
	} else {
		fmt.Printf("Commit %s revoked successfully.\n", args[0])
	}
}
