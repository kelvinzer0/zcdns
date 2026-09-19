package cmd

import (
	"fmt"
	"time"

	"zvault-cli/internal/workspace"
)

func cmdCommit(args []string) {
	msg := "update"
	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			msg = args[i+1]
		}
	}
	workspace.InitProject()
	working, _ := workspace.LoadWorking()
	
	hash := workspace.GenerateHash()
	parentHash := workspace.LoadHeadCommit()

	c := workspace.VaultCommit{
		ID:         workspace.GenerateHash() + workspace.GenerateHash(), // dummy uuid
		Hash:       hash,
		ParentHash: parentHash,
		Message:    msg,
		Timestamp:  time.Now(),
		Author:     "local",
	}

	var pairs []workspace.VaultKVPair
	for k, v := range working {
		pairs = append(pairs, workspace.VaultKVPair{
			ID:             workspace.GenerateHash() + workspace.GenerateHash(),
			CommitHash:     hash,
			KeyName:        k,
			EncryptedValue: v, // dummy encryption
		})
	}

	cData := map[string]any{"commit": c, "pairs": pairs}
	workspace.SaveCommitData(hash, cData)
	workspace.SaveHeadCommit(hash)
	workspace.SaveHead(working)

	fmt.Printf("Committed %s\n", hash)
}
