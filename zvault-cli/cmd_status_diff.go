package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func cmdStatus() {
	b, _ := os.ReadFile(".zvault/working.json")
	var working map[string]string
	json.Unmarshal(b, &working)

	hb, _ := os.ReadFile(".zvault/head.json")
	var head map[string]string
	json.Unmarshal(hb, &head)

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
	b, _ := os.ReadFile(".zvault/working.json")
	var working map[string]string
	json.Unmarshal(b, &working)

	hb, _ := os.ReadFile(".zvault/head.json")
	var head map[string]string
	json.Unmarshal(hb, &head)

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

func cmdRm(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: zvault rm <key>")
		return
	}
	initProject()
	b, _ := os.ReadFile(".zvault/working.json")
	var working map[string]string
	json.Unmarshal(b, &working)
	if working == nil {
		working = make(map[string]string)
	}

	key := args[0]
	if _, ok := working[key]; ok {
		delete(working, key)
		wb, _ := json.MarshalIndent(working, "", "  ")
		os.WriteFile(".zvault/working.json", wb, 0644)
		fmt.Printf("Removed %s\n", key)
	} else {
		fmt.Printf("Key %s not found\n", key)
	}
}
