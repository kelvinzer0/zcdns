#!/bin/bash

# handleGetVaultKV
sed -i '/commit := r.URL.Query().Get("commit")/!b;n;n;n;a\
	if !h.db.VerifyCommitOwnership(commit, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

# handleRevertVaultCommit
sed -i '/if req.RepoID == "" || req.CommitHash == "" {/!b;n;n;n;a\
	if !h.db.VerifyRepoOwnership(req.RepoID, subdomain) || !h.db.VerifyCommitOwnership(req.CommitHash, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

# handleSyncVault
sed -i '/if req.RepoID == "" {/!b;n;n;n;a\
	if !h.db.VerifyRepoOwnership(req.RepoID, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn\n\t}' backend/api/handler_vault.go

