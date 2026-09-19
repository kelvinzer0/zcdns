package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		if strings.Contains(shell, "zsh") {
			return "zsh"
		} else if strings.Contains(shell, "fish") {
			return "fish"
		} else if strings.Contains(shell, "bash") {
			return "bash"
		}
	}
	// Fallback for Windows or unknown
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	if os.Getenv("COMSPEC") != "" {
		return "cmd"
	}
	return "bash" // Default to bash
}

func getEnvFormat(shell string, key string, value string) string {
	switch shell {
	case "fish":
		return fmt.Sprintf("set -gx %s \"%s\";", key, value)
	case "powershell":
		return fmt.Sprintf("$env:%s=\"%s\"", key, value)
	case "cmd":
		return fmt.Sprintf("setx %s \"%s\"", key, value)
	case "nushell":
		return fmt.Sprintf("$env.%s = \"%s\"", key, value)
	default:
		// bash, zsh
		return fmt.Sprintf("export %s=\"%s\"", key, value)
	}
}

func cmdEnv(args []string) {
	if len(args) > 0 && args[0] == "install" {
		cmdEnvInstall()
		return
	}

	shell := detectShell()
	for i := 0; i < len(args); i++ {
		if (args[i] == "--shell" || args[i] == "-s") && i+1 < len(args) {
			shell = strings.ToLower(args[i+1])
			break
		}
	}

	b, err := os.ReadFile(".zvault/working.json")
	if err != nil {
		return
	}
	var working map[string]string
	json.Unmarshal(b, &working)
	if len(working) == 0 {
		return
	}

	for k, v := range working {
		fmt.Println(getEnvFormat(shell, k, v))
	}
}

func cmdEnvInstall() {
	shell := detectShell()
	home, _ := os.UserHomeDir()
	
	var profilePath string
	var hookCmd string

	switch shell {
	case "zsh":
		profilePath = filepath.Join(home, ".zshrc")
		hookCmd = "\n# ZVault Hook\neval \"$(zvault env)\"\n"
	case "fish":
		profilePath = filepath.Join(home, ".config", "fish", "config.fish")
		hookCmd = "\n# ZVault Hook\nzvault env --shell fish | source\n"
	case "bash":
		profilePath = filepath.Join(home, ".bashrc")
		hookCmd = "\n# ZVault Hook\neval \"$(zvault env)\"\n"
	case "powershell":
		fmt.Println("PowerShell auto-install is currently manual. Add the following to your $PROFILE:")
		fmt.Println("Invoke-Expression (zvault env --shell powershell)")
		return
	case "cmd":
		fmt.Println("CMD auto-install is not supported. Use zvault env --shell cmd and copy output.")
		return
	case "nushell":
		fmt.Println("Nushell auto-install is currently manual.")
		return
	default:
		profilePath = filepath.Join(home, ".bashrc")
		hookCmd = "\n# ZVault Hook\neval \"$(zvault env)\"\n"
	}

	// Create directory if needed (e.g. for fish)
	os.MkdirAll(filepath.Dir(profilePath), 0755)

	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening profile:", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(hookCmd); err != nil {
		fmt.Println("Error writing to profile:", err)
		return
	}

	fmt.Printf("Successfully installed zvault hook to %s\n", profilePath)
	fmt.Printf("Please restart your terminal or run: source %s\n", profilePath)
}
