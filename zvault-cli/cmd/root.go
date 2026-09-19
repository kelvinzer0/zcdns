package cmd

import (
	"fmt"
	"os"
)

const version = "0.1.0"

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
	fmt.Println("\nSecret Lifecycle & Environment:")
	fmt.Println("  env                   Output vault secrets as shell environment variables")
	fmt.Println("  env install           Add zvault hook to your shell profile (.bashrc, .zshrc, etc.)")
	fmt.Println("  revoke <commit>       Revert the vault back to a specific commit hash")
	fmt.Println("  destroy               Destroy and remove the local vault completely")
	fmt.Println("\nOther Commands:")
	fmt.Println("  version, -v           Show version information")
	fmt.Println("  help, -h              Show this help menu")
}

func Execute() {
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
	case "env":
		cmdEnv(os.Args[2:])
	case "rotate":
		fmt.Printf("zvault: '%s' is in active preview mode. Implementation coming soon!\n", command)
	default:
		fmt.Printf("zvault: '%s' is not a valid command.\n", command)
		os.Exit(1)
	}
}
