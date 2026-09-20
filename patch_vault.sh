#!/bin/bash

# Patch handleGetVaultBranches
sed -i '/repoID := r.URL.Query().Get("repo_id")/!b;n;n;n;a\
	if !h.db.VerifyRepoOwnership(repoID, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

# Patch handleGetVaultCommits
sed -i '/repoID := r.URL.Query().Get("repo_id")/!b;n;n;n;a\
	if !h.db.VerifyRepoOwnership(repoID, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

# Patch handleGetVaultKV
sed -i '/commit := r.URL.Query().Get("commit")/!b;n;n;n;a\
	if !h.db.VerifyCommitOwnership(commit, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

# Patch handleRevertVaultCommit
sed -i '/if req.RepoID == "" || req.CommitHash == "" {/!b;n;n;n;a\
	if !h.db.VerifyRepoOwnership(req.RepoID, subdomain) || !h.db.VerifyCommitOwnership(req.CommitHash, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

# Patch handleSyncVault
sed -i '/if req.RepoID == "" {/!b;n;n;n;a\
	if !h.db.VerifyRepoOwnership(req.RepoID, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

