package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestLocalOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zvault-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	initProject()
	if _, err := os.Stat(".zvault/commits"); os.IsNotExist(err) {
		t.Error("Expected .zvault/commits to be created")
	}

	cmdSet([]string{"TEST_KEY=test_value"})
	
	b, err := os.ReadFile(".zvault/working.json")
	if err != nil {
		t.Fatalf("Failed to read working.json: %v", err)
	}
	var working1 map[string]string
	json.Unmarshal(b, &working1)
	if working1["TEST_KEY"] != "test_value" {
		t.Errorf("Expected TEST_KEY=test_value, got %s", working1["TEST_KEY"])
	}

	cmdRm([]string{"TEST_KEY"})
	b2, _ := os.ReadFile(".zvault/working.json")
	var working2 map[string]string
	json.Unmarshal(b2, &working2)
	if _, exists := working2["TEST_KEY"]; exists {
		t.Error("Expected TEST_KEY to be removed")
	}
}
