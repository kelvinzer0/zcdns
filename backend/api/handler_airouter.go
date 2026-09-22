package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"zcdns-backend/db"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func extractSubdomainFromHost(host string) string {
	host = strings.Split(host, ":")[0]
	parts := strings.Split(host, ".")
	if len(parts) >= 4 && parts[1] == "router" {
		return parts[0]
	}
	return ""
}

// detectFormat returns "openai" or "anthropic" based on path
func detectFormat(path string) string {
	if strings.Contains(path, "/v1/messages") {
		return "anthropic"
	}
	return "openai"
}

// providerBaseURL returns the upstream base URL for a given provider
func providerBaseURL(provider, customBaseURL string) string {
	switch provider {
	case "anthropic":
		return "https://api.anthropic.com"
	case "groq":
		return "https://api.groq.com/openai"
	case "together":
		return "https://api.together.xyz"
	case "openrouter":
		return "https://openrouter.ai/api"
	case "custom":
		return strings.TrimRight(customBaseURL, "/")
	default: // openai
		return "https://api.openai.com"
	}
}

// isAnthropicProvider returns true if provider natively uses Anthropic format
func isAnthropicProvider(provider string) bool {
	return provider == "anthropic"
}

// buildUpstreamURL builds the full upstream URL
func buildUpstreamURL(provider, baseURL, inputFormat string) string {
	base := providerBaseURL(provider, baseURL)
	if isAnthropicProvider(provider) {
		return base + "/v1/messages"
	}
	return base + "/v1/chat/completions"
}

// ── Round-robin state (in-memory, per combo per subdomain) ───────────────────
var rrCounters = map[string]int{}

func rrNext(key string, n int) int {
	if n <= 1 {
		return 0
	}
	c := rrCounters[key]
	rrCounters[key] = (c + 1) % n
	return c
}

// ── Proxy core ────────────────────────────────────────────────────────────────

type proxyTarget struct {
	conn    *db.AIRouterConnection
	model   string
	provider string
}

// resolveTarget picks provider+model based on model string (alias → combo or direct)
func (h *APIHandler) resolveTargets(subdomain, modelStr, endpoint string, excludeConnIDs []int64) ([]proxyTarget, error) {
	// 1. Resolve alias
	resolved := h.db.ResolveAIRouterAlias(subdomain, modelStr)

	// 2. Check if it's a combo
	combo, err := h.db.GetAIRouterComboByName(subdomain, resolved)
	if err == nil && combo != nil {
		var models []string
		json.Unmarshal([]byte(combo.ModelsJSON), &models)
		if len(models) == 0 {
			return nil, fmt.Errorf("combo '%s' has no models", combo.Name)
		}

		switch combo.Strategy {
		case "round_robin", "round_robin_sticky":
			idx := rrNext(subdomain+"/"+combo.Name, len(models))
			m := models[idx]
			return h.resolveTargets(subdomain, m, endpoint, excludeConnIDs)
		default: // fallback: return all in order
			var targets []proxyTarget
			for _, m := range models {
				t, err := h.resolveTargets(subdomain, m, endpoint, excludeConnIDs)
				if err == nil {
					targets = append(targets, t...)
				}
			}
			return targets, nil
		}
	}

	// 3. Direct "provider/model" format
	var provider, model string
	if strings.Contains(resolved, "/") {
		parts := strings.SplitN(resolved, "/", 2)
		provider, model = parts[0], parts[1]
	} else {
		// Guess provider from model name
		s := strings.ToLower(resolved)
		switch {
		case strings.HasPrefix(s, "claude"):
			provider = "anthropic"
		case strings.HasPrefix(s, "llama"), strings.HasPrefix(s, "mixtral"), strings.HasPrefix(s, "gemma"):
			provider = "groq"
		default:
			provider = "openai"
		}
		model = resolved
	}

	// 4. Find an active connection
	h.db.ReactivateExpiredAIRouterConnections()
	conn, err := h.db.GetActiveAIRouterConnection(subdomain, provider, excludeConnIDs)
	if err != nil {
		return nil, fmt.Errorf("no active connection for provider '%s' (add one in Dashboard → AI Router → Connections)", provider)
	}

	return []proxyTarget{{conn: conn, model: model, provider: provider}}, nil
}

// forwardRequest sends one HTTP request to an upstream provider and streams/copies back
func forwardRequest(w http.ResponseWriter, bodyBytes []byte, target proxyTarget, inputFormat, outputFormat string) error {
	upstreamURL := buildUpstreamURL(target.provider, target.conn.BaseURL, inputFormat)

	// Adjust model in body
	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)
	reqBody["model"] = target.model
	// Remove anthropic-only fields if going to openai
	if !isAnthropicProvider(target.provider) {
		delete(reqBody, "max_tokens")
		if _, ok := reqBody["messages"]; !ok {
			reqBody["messages"] = []interface{}{}
		}
	}
	newBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(newBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if isAnthropicProvider(target.provider) {
		req.Header.Set("x-api-key", target.conn.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+target.conn.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("rate_limited:%d", target.conn.ID)
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("X-Router-Provider", target.provider)
	w.Header().Set("X-Router-Model", target.model)
	w.WriteHeader(resp.StatusCode)

	isStream, _ := reqBody["stream"].(bool)
	if isStream {
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}
	return nil
}

// handleAIRouterProxy is the core proxy entrypoint
func (h *APIHandler) handleAIRouterProxy(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Resolve subdomain
	subdomain := extractSubdomainFromHost(r.Host)
	if subdomain == "" {
		parts := strings.Split(r.URL.Path, "/")
		for i, p := range parts {
			if p == "airouter" && i+1 < len(parts) {
				subdomain = parts[i+1]
				break
			}
		}
	}
	if subdomain == "" {
		writeJSONError(w, http.StatusBadRequest, "could not determine subdomain from host or path")
		return
	}

	// Ensure config exists
	cfg, err := h.db.EnsureAIRouterConfig(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "router not configured")
		return
	}

	// Detect format
	inputFormat := cfg.InputFormat
	if inputFormat == "auto" {
		inputFormat = detectFormat(r.URL.Path)
	}

	// Read body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)
	requestedModel, _ := reqBody["model"].(string)
	if requestedModel == "" {
		requestedModel = "gpt-4o-mini"
	}

	// Resolve targets with fallback loop
	var excludeConnIDs []int64
	var lastErr string

	for attempts := 0; attempts < 5; attempts++ {
		targets, err := h.resolveTargets(subdomain, requestedModel, r.URL.Path, excludeConnIDs)
		if err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, err.Error())
			return
		}

		// Try each target in order (for fallback combo)
		var forwarded bool
		for _, target := range targets {
			err := forwardRequest(w, bodyBytes, target, inputFormat, cfg.OutputFormat)
			if err != nil && strings.HasPrefix(err.Error(), "rate_limited:") {
				// Mark rate limited and try next
				idStr := strings.TrimPrefix(err.Error(), "rate_limited:")
				if id, e := strconv.ParseInt(idStr, 10, 64); e == nil {
					h.db.MarkAIRouterConnectionRateLimited(id)
					excludeConnIDs = append(excludeConnIDs, id)
				}
				lastErr = "rate limited, trying next connection"
				continue
			}
			if err != nil {
				lastErr = err.Error()
				continue
			}
			forwarded = true
			break
		}

		if forwarded {
			return
		}
	}

	writeJSONError(w, http.StatusServiceUnavailable, "all connections failed: "+lastErr)
}

func (h *APIHandler) handleAIRouterOpenAI(w http.ResponseWriter, r *http.Request) {
	h.handleAIRouterProxy(w, r)
}

func (h *APIHandler) handleAIRouterAnthropic(w http.ResponseWriter, r *http.Request) {
	h.handleAIRouterProxy(w, r)
}

// ── Config endpoints ──────────────────────────────────────────────────────────

func (h *APIHandler) handleGetAIRouterConfig(w http.ResponseWriter, r *http.Request, subdomain string) {
	cfg, _ := h.db.EnsureAIRouterConfig(subdomain)
	conns, _ := h.db.GetAIRouterConnections(subdomain)
	combos, _ := h.db.GetAIRouterCombos(subdomain)
	aliases, _ := h.db.GetAIRouterAliases(subdomain)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":      cfg,
		"connections": conns,
		"combos":      combos,
		"aliases":     aliases,
	})
}

func (h *APIHandler) handleSaveAIRouterConfig(w http.ResponseWriter, r *http.Request, subdomain string) {
	var body struct {
		InputFormat  string `json:"input_format"`
		OutputFormat string `json:"output_format"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	h.db.EnsureAIRouterConfig(subdomain)
	h.db.UpdateAIRouterConfig(subdomain, body.InputFormat, body.OutputFormat)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Connection CRUD ───────────────────────────────────────────────────────────

func (h *APIHandler) handleAddAIRouterConnection(w http.ResponseWriter, r *http.Request, subdomain string) {
	var conn db.AIRouterConnection
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	conn.Subdomain = subdomain
	id, err := h.db.AddAIRouterConnection(&conn)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

func (h *APIHandler) handleDeleteAIRouterConnection(w http.ResponseWriter, r *http.Request, subdomain string) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	h.db.DeleteAIRouterConnection(subdomain, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Combo CRUD ────────────────────────────────────────────────────────────────

func (h *APIHandler) handleUpsertAIRouterCombo(w http.ResponseWriter, r *http.Request, subdomain string) {
	var combo db.AIRouterCombo
	if err := json.NewDecoder(r.Body).Decode(&combo); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	combo.Subdomain = subdomain
	if err := h.db.UpsertAIRouterCombo(&combo); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleDeleteAIRouterCombo(w http.ResponseWriter, r *http.Request, subdomain string) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	h.db.DeleteAIRouterCombo(subdomain, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Alias CRUD ────────────────────────────────────────────────────────────────

func (h *APIHandler) handleUpsertAIRouterAlias(w http.ResponseWriter, r *http.Request, subdomain string) {
	var alias db.AIRouterAlias
	if err := json.NewDecoder(r.Body).Decode(&alias); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	alias.Subdomain = subdomain
	if err := h.db.UpsertAIRouterAlias(&alias); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleDeleteAIRouterAlias(w http.ResponseWriter, r *http.Request, subdomain string) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	h.db.DeleteAIRouterAlias(subdomain, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
