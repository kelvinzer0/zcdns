package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"zcdns-backend/db"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func estimateInputTokens(bodyBytes []byte) int {
	if len(bodyBytes) == 0 {
		return 10
	}
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return len(bodyBytes) / 4
	}
	totalChars := 0
	// OpenAI messages / Anthropic messages
	if msgs, ok := body["messages"].([]interface{}); ok {
		for _, m := range msgs {
			if mObj, ok := m.(map[string]interface{}); ok {
				if content, ok := mObj["content"].(string); ok {
					totalChars += len(content)
				} else if contentArr, ok := mObj["content"].([]interface{}); ok {
					for _, part := range contentArr {
						if partObj, ok := part.(map[string]interface{}); ok {
							if text, ok := partObj["text"].(string); ok {
								totalChars += len(text)
							}
						}
					}
				}
			}
		}
	}
	// System prompt (Anthropic or OpenAI)
	if sys, ok := body["system"].(string); ok {
		totalChars += len(sys)
	}
	// Prompt (legacy completions)
	if p, ok := body["prompt"].(string); ok {
		totalChars += len(p)
	}

	if totalChars == 0 {
		totalChars = len(bodyBytes)
	}

	tokens := totalChars / 4
	if tokens < 10 {
		tokens = 10
	}
	return tokens
}

func getDefaultContextSize(modelName string) int {
	s := strings.ToLower(modelName)
	switch {
	case strings.Contains(s, "claude-3-5") || strings.Contains(s, "claude-3"):
		return 200000
	case strings.Contains(s, "gpt-4o") || strings.Contains(s, "o1") || strings.Contains(s, "o3"):
		return 128000
	case strings.Contains(s, "llama-3.1") || strings.Contains(s, "llama-3.3") || strings.Contains(s, "llama-3.2"):
		return 128000
	case strings.Contains(s, "llama-3-"):
		return 8192
	case strings.Contains(s, "gemma-2") || strings.Contains(s, "gemma2"):
		return 8192
	case strings.Contains(s, "mistral-large"):
		return 128000
	case strings.Contains(s, "mixtral"):
		return 32768
	case strings.Contains(s, "gpt-4-turbo"):
		return 128000
	case strings.Contains(s, "gpt-4-32k"):
		return 32768
	case strings.Contains(s, "gpt-4"):
		return 8192
	case strings.Contains(s, "gpt-3.5-turbo-16k"):
		return 16384
	case strings.Contains(s, "gpt-3.5"):
		return 4096
	default:
		return 128000
	}
}

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
	conn     *db.AIRouterConnection
	model    string
	provider string
}

// resolveTargets picks provider+model based on model string (alias → combo or direct)
func (h *APIHandler) resolveTargets(subdomain, modelStr, endpoint string, bodyBytes []byte, excludeConnIDs []int64) ([]proxyTarget, error) {
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
			return h.resolveTargets(subdomain, m, endpoint, bodyBytes, excludeConnIDs)

		case "smart_context":
			tokens := estimateInputTokens(bodyBytes)
			type candidate struct {
				model   string
				ctxSize int
			}
			var candidates []candidate
			for _, m := range models {
				ctx := 0
				if alias, err := h.db.GetAIRouterAlias(subdomain, m); err == nil && alias != nil && alias.ContextSize > 0 {
					ctx = alias.ContextSize
				} else {
					ctx = getDefaultContextSize(m)
				}
				candidates = append(candidates, candidate{model: m, ctxSize: ctx})
			}
			// Sort candidates by context size ascending
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].ctxSize < candidates[j].ctxSize
			})

			// Split into models that can comfortably fit the context vs smaller ones
			var suitable []string
			var tooSmall []string
			for _, c := range candidates {
				if c.ctxSize >= tokens {
					suitable = append(suitable, c.model)
				} else {
					tooSmall = append(tooSmall, c.model)
				}
			}

			// Smallest capable model first, then larger models as fallback
			var ordered []string
			if len(suitable) > 0 {
				ordered = append(suitable, tooSmall...)
			} else {
				// All models are too small: try the largest available first
				for i := len(candidates) - 1; i >= 0; i-- {
					ordered = append(ordered, candidates[i].model)
				}
			}

			var targets []proxyTarget
			for _, m := range ordered {
				t, err := h.resolveTargets(subdomain, m, endpoint, bodyBytes, excludeConnIDs)
				if err == nil {
					targets = append(targets, t...)
				}
			}
			return targets, nil

		default: // fallback: return all in order
			var targets []proxyTarget
			for _, m := range models {
				t, err := h.resolveTargets(subdomain, m, endpoint, bodyBytes, excludeConnIDs)
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
		targets, err := h.resolveTargets(subdomain, requestedModel, r.URL.Path, bodyBytes, excludeConnIDs)
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
