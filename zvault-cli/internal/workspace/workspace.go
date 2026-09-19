package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VaultCommit struct {
	ID         string    `json:"id"`
	RepoID     string    `json:"repo_id"`
	Hash       string    `json:"hash"`
	ParentHash string    `json:"parent_hash"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Author     string    `json:"author"`
}

type VaultKVPair struct {
	ID             string `json:"id"`
	CommitHash     string `json:"commit_hash"`
	KeyName        string `json:"key_name"`
	EncryptedValue string `json:"encrypted_value"`
}

func GetProjectDir() string {
	return ".zvault"
}

func InitProject() {
	os.MkdirAll(".zvault/commits", 0755)
	if _, err := os.Stat(".zvault/head.json"); os.IsNotExist(err) {
		os.WriteFile(".zvault/head.json", []byte("{}"), 0644)
	}
	if _, err := os.Stat(".zvault/working.json"); os.IsNotExist(err) {
		os.WriteFile(".zvault/working.json", []byte("{}"), 0644)
	}
}

func LoadWorking() (map[string]string, error) {
	b, err := os.ReadFile(".zvault/working.json")
	if err != nil {
		return nil, err
	}
	var working map[string]string
	err = json.Unmarshal(b, &working)
	if working == nil {
		working = make(map[string]string)
	}
	return working, err
}

func SaveWorking(working map[string]string) error {
	wb, err := json.MarshalIndent(working, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(".zvault/working.json", wb, 0644)
}

func LoadHead() (map[string]string, error) {
	b, err := os.ReadFile(".zvault/head.json")
	if err != nil {
		return nil, err
	}
	var head map[string]string
	err = json.Unmarshal(b, &head)
	if head == nil {
		head = make(map[string]string)
	}
	return head, err
}

func SaveHead(head map[string]string) error {
	wb, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(".zvault/head.json", wb, 0644)
}

func LoadHeadCommit() string {
	ph, err := os.ReadFile(".zvault/head_commit")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(ph))
}

func SaveHeadCommit(hash string) error {
	return os.WriteFile(".zvault/head_commit", []byte(hash), 0644)
}

func GenerateHash() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)[:8]
}

func SaveCommitData(hash string, data any) error {
	cb, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(".zvault/commits", hash+".json"), cb, 0644)
}

func DestroyProject() {
	os.RemoveAll(".zvault")
}
