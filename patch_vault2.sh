#!/bin/bash

# handleGetVaultBranches
sed -i 's/if repoID == "" {/if repoID == "" {\n\t\twriteJSONError(w, http.StatusBadRequest, "repo_id is required")\n\t\treturn\n\t}\n\tif !h.db.VerifyRepoOwnership(repoID, subdomain) {\n\t\twriteJSONError(w, http.StatusForbidden, "Akses ditolak")\n\t\treturn/g' backend/api/handler_vault.go

