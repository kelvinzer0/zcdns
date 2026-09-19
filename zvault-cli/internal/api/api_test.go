package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"zvault-cli/internal/config"
)

func TestGetRepoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/vault/init" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"mock-repo-id"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origBaseURL := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = origBaseURL }()

	cfg := &config.Config{
		Token:     "mock-token",
		Subdomain: "mock-sub",
	}

	repoID, err := GetRepoID(cfg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if repoID != "mock-repo-id" {
		t.Errorf("Expected mock-repo-id, got %s", repoID)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "zvault-config-*")
	defer os.RemoveAll(tmpDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	os.MkdirAll(filepath.Join(tmpDir, ".zvault"), 0755)
	
	testCfg := config.Config{Token: "token123", Subdomain: "sub123"}
	b, _ := json.Marshal(testCfg)
	os.WriteFile(filepath.Join(tmpDir, ".zvault", "config.json"), b, 0644)

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Token != "token123" || cfg.Subdomain != "sub123" {
		t.Errorf("Config mismatch. Got %+v", cfg)
	}
}
