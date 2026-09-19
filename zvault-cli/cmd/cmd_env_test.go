package cmd

import (
	"os"
	"testing"
)

func TestDetectShell(t *testing.T) {
	origShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", origShell)

	os.Setenv("SHELL", "/bin/zsh")
	if shell := detectShell(); shell != "zsh" {
		t.Errorf("Expected zsh, got %s", shell)
	}

	os.Setenv("SHELL", "/usr/bin/fish")
	if shell := detectShell(); shell != "fish" {
		t.Errorf("Expected fish, got %s", shell)
	}

	os.Setenv("SHELL", "/bin/bash")
	if shell := detectShell(); shell != "bash" {
		t.Errorf("Expected bash, got %s", shell)
	}
}

func TestGetEnvFormat(t *testing.T) {
	tests := []struct {
		shell    string
		key      string
		val      string
		expected string
	}{
		{"bash", "API_KEY", "123", "export API_KEY=\"123\""},
		{"zsh", "API_KEY", "123", "export API_KEY=\"123\""},
		{"fish", "API_KEY", "123", "set -gx API_KEY \"123\";"},
		{"powershell", "API_KEY", "123", "$env:API_KEY=\"123\""},
		{"cmd", "API_KEY", "123", "setx API_KEY \"123\""},
		{"nushell", "API_KEY", "123", "$env.API_KEY = \"123\""},
	}

	for _, tt := range tests {
		result := getEnvFormat(tt.shell, tt.key, tt.val)
		if result != tt.expected {
			t.Errorf("For shell %s, expected '%s', got '%s'", tt.shell, tt.expected, result)
		}
	}
}
