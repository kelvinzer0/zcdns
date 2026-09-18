package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func printHelp() {
	fmt.Printf("zvault v%s - Git-style version control for secrets\n\n", version)
	fmt.Println("Usage: zvault <command> [arguments]")
	fmt.Println("\nAuthentication & Setup:")
	fmt.Println("  login                 Authenticate CLI with your ZCDNS account")
	fmt.Println("  init                  Initialize a new secrets vault")
	fmt.Println("  clone <url>           Clone a remote vault repository")
	fmt.Println("\nRepository & Workflow Commands:")
	fmt.Println("  status                Show working tree status & modified secrets")
	fmt.Println("  set <key=value>       Add or update a secret value")
	fmt.Println("  rm <key>              Remove a secret from vault")
	fmt.Println("  commit -m <msg>       Record changes to the vault repository")
	fmt.Println("  log                   Show commit logs")
	fmt.Println("  show <commit>         Show metadata and changes in a commit")
	fmt.Println("  diff [commit]         Show changes between commits (concealed values)")
	fmt.Println("  push                  Push local commits to remote vault")
	fmt.Println("  pull                  Fetch and integrate remote changes")
	fmt.Println("  branch [name]         List or create branches")
	fmt.Println("  switch <branch>       Switch branches")
	fmt.Println("\nSecret Lifecycle & Rotation Commands:")
	fmt.Println("  rotate <secret>       Rotate a specific secret key")
	fmt.Println("  revoke <version>      Revoke a secret version")
	fmt.Println("  destroy <version>     Permanently destroy decryptability of older versions")
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
	case "login", "init", "clone", "status", "set", "rm", "commit", "log", "show", "diff", "push", "pull", "branch", "switch", "destroy", "rotate", "revoke":
		fmt.Printf("zvault: '%s' is in active preview mode. Implementation coming soon!\n", command)
	default:
		fmt.Printf("zvault: '%s' is not a valid command. Run 'zvault --help' for usage.\n", command)
		os.Exit(1)
	}
}
