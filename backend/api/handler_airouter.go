package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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
	if customBaseURL != "" {
		return strings.TrimRight(customBaseURL, "/")
	}
	switch provider {
	case "anthropic":
		return "https://api.anthropic.com"
	case "groq":
		return "https://api.groq.com/openai"
	case "together":
		return "https://api.together.xyz"
	case "openrouter":
		return "https://openrouter.ai/api"
	case "deepseek":
		return "https://api.deepseek.com"
	default: // openai
		return "https://api.openai.com"
	}
}

// isAnthropicProvider returns true if provider natively uses Anthropic format
func isAnthropicProvider(provider string) bool {
	return provider == "anthropic"
}

// isAnthropicTarget returns true if target connection uses Anthropic protocol
func isAnthropicTarget(target proxyTarget) bool {
	if target.conn != nil && target.conn.APIType != "" {
		return target.conn.APIType == "anthropic"
	}
	return isAnthropicProvider(target.provider)
}

// buildUpstreamURL builds the full upstream URL
func buildUpstreamURL(provider, apiType, baseURL, inputFormat string) string {
	base := providerBaseURL(provider, baseURL)
	isAnthropic := (apiType == "anthropic" || (apiType == "" && isAnthropicProvider(provider)))
	if isAnthropic {
		if strings.HasSuffix(base, "/v1/messages") || strings.HasSuffix(base, "/messages") {
			return base
		}
		if strings.HasSuffix(base, "/v1") {
			return base + "/messages"
		}
		return base + "/v1/messages"
	}
	if strings.HasSuffix(base, "/v1/chat/completions") || strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
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

	// 3. Find target connection and model
	h.db.ReactivateExpiredAIRouterConnections()
	activeConns, err := h.db.GetActiveAIRouterConnectionsUnmasked(subdomain, excludeConnIDs)
	if err != nil || len(activeConns) == 0 {
		return nil, fmt.Errorf("no active connection configured (add one in Dashboard → AI Router → Connections)")
	}

	// Phase A: Exact model match in any active connection's models_json
	// e.g. a connection configured with ["deepseek-ai/deepseek-v4.1-flash"] or ["gpt-4o"]
	for _, c := range activeConns {
		var models []string
		_ = json.Unmarshal([]byte(c.ModelsJSON), &models)
		for _, m := range models {
			if strings.EqualFold(m, resolved) {
				connCopy := c
				return []proxyTarget{{conn: &connCopy, model: m, provider: c.Provider}}, nil
			}
		}
	}

	// Phase B: Connection name or provider prefix match (e.g. "groq/llama-3.3-70b-versatile" or "custom/deepseek-v4")
	if strings.Contains(resolved, "/") {
		parts := strings.SplitN(resolved, "/", 2)
		prefix := strings.ToLower(parts[0])
		subModel := parts[1]

		// Check if prefix matches a connection's provider or name
		for _, c := range activeConns {
			if strings.EqualFold(c.Provider, prefix) || strings.EqualFold(c.Name, parts[0]) {
				var models []string
				_ = json.Unmarshal([]byte(c.ModelsJSON), &models)
				for _, m := range models {
					if strings.EqualFold(m, subModel) || strings.EqualFold(m, resolved) {
						connCopy := c
						return []proxyTarget{{conn: &connCopy, model: m, provider: c.Provider}}, nil
					}
				}
				connCopy := c
				return []proxyTarget{{conn: &connCopy, model: subModel, provider: c.Provider}}, nil
			}
		}

		// Known standard providers
		standardProviders := []string{"openai", "anthropic", "deepseek", "groq", "together", "openrouter", "custom"}
		for _, sp := range standardProviders {
			if prefix == sp {
				for _, c := range activeConns {
					if strings.EqualFold(c.Provider, sp) {
						connCopy := c
						return []proxyTarget{{conn: &connCopy, model: subModel, provider: c.Provider}}, nil
					}
				}
				return nil, fmt.Errorf("no active connection for provider '%s' (add one in Dashboard → AI Router → Connections)", prefix)
			}
		}
	}

	// Phase C: Model heuristics based on keywords/vendor prefixes
	// (Note: resolved may contain org paths like "deepseek-ai/deepseek-v4.1-flash" or "meta-llama/Llama-3.1-70B")
	s := strings.ToLower(resolved)
	var preferredProviders []string
	switch {
	case strings.Contains(s, "claude"):
		preferredProviders = []string{"anthropic", "openrouter", "custom"}
	case strings.Contains(s, "deepseek"):
		preferredProviders = []string{"deepseek", "custom", "openrouter", "together"}
	case strings.Contains(s, "llama"), strings.Contains(s, "mixtral"), strings.Contains(s, "gemma"), strings.Contains(s, "qwen"):
		preferredProviders = []string{"groq", "together", "openrouter", "custom"}
	case strings.HasPrefix(s, "gpt-"), strings.HasPrefix(s, "o1"), strings.HasPrefix(s, "o3"), strings.HasPrefix(s, "chatgpt"):
		preferredProviders = []string{"openai", "openrouter", "custom"}
	default:
		preferredProviders = []string{"custom", "openrouter", "openai"}
	}

	for _, pref := range preferredProviders {
		for _, c := range activeConns {
			if strings.EqualFold(c.Provider, pref) {
				connCopy := c
				return []proxyTarget{{conn: &connCopy, model: resolved, provider: c.Provider}}, nil
			}
		}
	}

	// Phase D: If user has a "custom" or "openrouter" connection, route any unmatched model to it
	for _, c := range activeConns {
		if c.Provider == "custom" || c.Provider == "openrouter" {
			connCopy := c
			return []proxyTarget{{conn: &connCopy, model: resolved, provider: c.Provider}}, nil
		}
	}

	// Phase E: Single active connection fallback - if only 1 connection exists, route to it
	if len(activeConns) == 1 {
		connCopy := activeConns[0]
		return []proxyTarget{{conn: &connCopy, model: resolved, provider: connCopy.Provider}}, nil
	}

	// Phase F: Default to first active connection
	connCopy := activeConns[0]
	return []proxyTarget{{conn: &connCopy, model: resolved, provider: connCopy.Provider}}, nil
}

// forwardRequest sends one HTTP request to an upstream provider and streams/copies back
func forwardRequest(w http.ResponseWriter, bodyBytes []byte, target proxyTarget, inputFormat, outputFormat string) error {
	apiType := ""
	baseURL := ""
	apiKey := ""
	if target.conn != nil {
		apiType = target.conn.APIType
		baseURL = target.conn.BaseURL
		apiKey = target.conn.APIKey
	}
	upstreamURL := buildUpstreamURL(target.provider, apiType, baseURL, inputFormat)
	isAnthropic := isAnthropicTarget(target)

	// Adjust model in body
	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)
	reqBody["model"] = target.model
	// Remove anthropic-only fields if going to openai
	if !isAnthropic {
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
	if isAnthropic {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
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
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")
	w.Header().Set("Access-Control-Max-Age", "86400")

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

	// Client authentication: if user has generated any API keys, validate incoming request
	if h.db.CountAIRouterUserKeys(subdomain) > 0 {
		clientKey := extractClientAPIKey(r)
		if !h.db.ValidateAIRouterUserKey(subdomain, clientKey) {
			writeJSONError(w, http.StatusUnauthorized, "invalid or missing API key (provide valid key in Authorization: Bearer <key> or x-api-key header)")
			return
		}
	}

	// Auto-detect format from request; output format always matches input format
	inputFormat := detectFormat(r.URL.Path)
	outputFormat := inputFormat

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
			err := forwardRequest(w, bodyBytes, target, inputFormat, outputFormat)
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
	userKeys, _ := h.db.GetAIRouterUserKeys(subdomain)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":      cfg,
		"connections": conns,
		"combos":      combos,
		"aliases":     aliases,
		"user_keys":   userKeys,
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

type ConnectionTestRequest struct {
	ID       int64  `json:"id,omitempty"`
	Provider string `json:"provider"`
	APIType  string `json:"api_type"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
}

func testProviderConnection(provider, apiType, apiKey, customBaseURL string) (bool, int64, []string, string) {
	start := time.Now()
	baseURL := providerBaseURL(provider, customBaseURL)
	isAnthropic := (apiType == "anthropic" || (apiType == "" && provider == "anthropic"))

	client := &http.Client{Timeout: 12 * time.Second}
	var testURL string
	var httpReq *http.Request
	var err error

	if isAnthropic {
		testURL = strings.TrimRight(baseURL, "/") + "/v1/models"
		if strings.HasSuffix(baseURL, "/v1") {
			testURL = strings.TrimRight(baseURL, "/") + "/models"
		}
		httpReq, err = http.NewRequest(http.MethodGet, testURL, nil)
		if err != nil {
			return false, time.Since(start).Milliseconds(), nil, err.Error()
		}
		httpReq.Header.Set("x-api-key", apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	} else {
		// OpenAI compatible
		testURL = strings.TrimRight(baseURL, "/") + "/v1/models"
		if strings.HasSuffix(baseURL, "/v1") {
			testURL = strings.TrimRight(baseURL, "/") + "/models"
		}
		httpReq, err = http.NewRequest(http.MethodGet, testURL, nil)
		if err != nil {
			return false, time.Since(start).Milliseconds(), nil, err.Error()
		}
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return false, latency, nil, fmt.Sprintf("Network connection error to %s: %v", baseURL, err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, latency, nil, fmt.Sprintf("Authentication failed (%d): Invalid API key", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		return false, latency, nil, fmt.Sprintf("Upstream returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	// Parse models if available
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	_ = json.Unmarshal(respBytes, &result)

	var models []string
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	for _, m := range result.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		if name != "" {
			models = append(models, name)
		}
	}
	sort.Strings(models)

	msg := "Connected successfully!"
	if len(models) > 0 {
		msg = fmt.Sprintf("Connected successfully! Found %d available models.", len(models))
	}

	return true, latency, models, msg
}

func (h *APIHandler) handleTestAIRouterConnection(w http.ResponseWriter, r *http.Request, subdomain string) {
	var body ConnectionTestRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.ID > 0 && (body.APIKey == "" || strings.Contains(body.APIKey, "...")) {
		conn, err := h.db.GetAIRouterConnectionByID(subdomain, body.ID)
		if err == nil && conn != nil {
			body.APIKey = conn.APIKey
			if body.Provider == "" {
				body.Provider = conn.Provider
			}
			if body.APIType == "" {
				body.APIType = conn.APIType
			}
			if body.BaseURL == "" {
				body.BaseURL = conn.BaseURL
			}
		}
	}

	if body.APIKey == "" {
		writeJSONError(w, http.StatusBadRequest, "API Key is required to test connection")
		return
	}

	success, latency, models, msg := testProviderConnection(body.Provider, body.APIType, body.APIKey, body.BaseURL)
	if !success {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":    false,
			"latency_ms": latency,
			"error":      msg,
		})
		return
	}

	if body.ID > 0 && len(models) > 0 {
		mJSON, _ := json.Marshal(models)
		_ = h.db.UpdateAIRouterConnectionModels(subdomain, body.ID, string(mJSON))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"latency_ms": latency,
		"models":     models,
		"message":    msg,
	})
}

func (h *APIHandler) handleFetchAIRouterModels(w http.ResponseWriter, r *http.Request, subdomain string) {
	var body ConnectionTestRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	if body.ID > 0 && (body.APIKey == "" || strings.Contains(body.APIKey, "...")) {
		conn, err := h.db.GetAIRouterConnectionByID(subdomain, body.ID)
		if err == nil && conn != nil {
			body.APIKey = conn.APIKey
			if body.Provider == "" {
				body.Provider = conn.Provider
			}
			if body.APIType == "" {
				body.APIType = conn.APIType
			}
			if body.BaseURL == "" {
				body.BaseURL = conn.BaseURL
			}
		}
	}

	success, _, models, msg := testProviderConnection(body.Provider, body.APIType, body.APIKey, body.BaseURL)
	if !success {
		writeJSONError(w, http.StatusBadRequest, msg)
		return
	}

	if body.ID > 0 && len(models) > 0 {
		mJSON, _ := json.Marshal(models)
		_ = h.db.UpdateAIRouterConnectionModels(subdomain, body.ID, string(mJSON))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models": models,
	})
}

func (h *APIHandler) handleUpdateConnectionModels(w http.ResponseWriter, r *http.Request, subdomain string) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}

	modelsJSON, _ := json.Marshal(body.Models)
	if err := h.db.UpdateAIRouterConnectionModels(subdomain, id, string(modelsJSON)); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "models": body.Models})
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

// ── Proxy Client API Key Authentication & Management ─────────────────────────

func extractClientAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if key := r.Header.Get("x-api-key"); key != "" {
		return strings.TrimSpace(key)
	}
	if key := r.Header.Get("api-key"); key != "" {
		return strings.TrimSpace(key)
	}
	return ""
}

func generateProxyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "zck_" + hex.EncodeToString(b)
}

func (h *APIHandler) handleGetAIRouterUserKeys(w http.ResponseWriter, r *http.Request, subdomain string) {
	keys, err := h.db.GetAIRouterUserKeys(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if keys == nil {
		keys = []db.AIRouterUserKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (h *APIHandler) handleCreateAIRouterUserKey(w http.ResponseWriter, r *http.Request, subdomain string) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "Default Key"
	}
	keyVal := generateProxyKey()
	k, err := h.db.CreateAIRouterUserKey(subdomain, body.Name, keyVal)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, k)
}

func (h *APIHandler) handleDeleteAIRouterUserKey(w http.ResponseWriter, r *http.Request, subdomain string) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.db.DeleteAIRouterUserKey(subdomain, id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
