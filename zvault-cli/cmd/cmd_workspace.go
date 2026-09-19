package cmd

import (
	"fmt"
	"strings"

	"zvault-cli/internal/workspace"
)

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
	workspace.InitProject()
	working, _ := workspace.LoadWorking()
	working[parts[0]] = parts[1]
	workspace.SaveWorking(working)
	fmt.Printf("Set %s\n", parts[0])
}

func cmdRm(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: zvault rm <key>")
		return
	}
	workspace.InitProject()
	working, _ := workspace.LoadWorking()

	key := args[0]
	if _, ok := working[key]; ok {
		delete(working, key)
		workspace.SaveWorking(working)
		fmt.Printf("Removed %s\n", key)
	} else {
		fmt.Printf("Key %s not found\n", key)
	}
}

func cmdDestroy() {
	fmt.Println("Destroying vault...")
	workspace.DestroyProject()
	fmt.Println("Local vault destroyed.")
}
