package api

import (
	"net/http"
	"zcdns-backend/db"
)

func (h *APIHandler) handleInitVault(w http.ResponseWriter, r *http.Request, subdomain string) {
	repo, err := h.db.InitVault(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to init vault: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *APIHandler) handleGetVaultBranches(w http.ResponseWriter, r *http.Request, subdomain string) {
	repoID := r.URL.Query().Get("repo_id")
	if repoID == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_id is required")
		return
	}
	branches, err := h.db.GetBranches(repoID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get branches: "+err.Error())
		return
	}
	if branches == nil {
		branches = make([]db.VaultBranch, 0)
	}
	writeJSON(w, http.StatusOK, branches)
}

func (h *APIHandler) handleGetVaultCommits(w http.ResponseWriter, r *http.Request, subdomain string) {
	repoID := r.URL.Query().Get("repo_id")
	if repoID == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_id is required")
		return
	}
	commits, err := h.db.GetCommits(repoID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get commits: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, commits)
}
