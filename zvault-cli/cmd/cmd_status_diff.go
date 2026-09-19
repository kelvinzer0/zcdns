package cmd

import (
	"fmt"
	"zvault-cli/internal/workspace"
)

func cmdStatus() {
	working, _ := workspace.LoadWorking()
	head, _ := workspace.LoadHead()

	fmt.Println("On branch main")
	fmt.Println("Changes not staged for commit:")
	
	hasChanges := false
	for k, v := range working {
		if hv, ok := head[k]; ok {
			if v != hv {
				fmt.Printf("  modified: %s\n", k)
				hasChanges = true
			}
		} else {
			fmt.Printf("  added:    %s\n", k)
			hasChanges = true
		}
	}
	for k := range head {
		if _, ok := working[k]; !ok {
			fmt.Printf("  deleted:  %s\n", k)
			hasChanges = true
		}
	}

	if !hasChanges {
		fmt.Println("nothing to commit, working tree clean")
	}
}

func cmdDiff() {
	working, _ := workspace.LoadWorking()
	head, _ := workspace.LoadHead()

	for k, v := range working {
		hv, ok := head[k]
		if ok {
			if v != hv {
				fmt.Printf("~ %s:\n  - %s\n  + %s\n", k, hv, v)
			}
		} else {
			fmt.Printf("+ %s: %s\n", k, v)
		}
	}
	for k, hv := range head {
		if _, ok := working[k]; !ok {
			fmt.Printf("- %s: %s\n", k, hv)
		}
	}
}
