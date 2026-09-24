package api

import (
	"bufio"
	"bytes"
	"context"
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

	"github.com/google/uuid"

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
var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"proxy-connection":    true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"content-length":      true,
	"content-encoding":    true,
}

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

	// Parse and normalize request body
	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)

	// Extract messages: handle standard messages or OpenWebUI's user_message
	var messages []interface{}
	if rawMsgs, ok := reqBody["messages"].([]interface{}); ok && len(rawMsgs) > 0 {
		messages = rawMsgs
	} else if userMsg, ok := reqBody["user_message"].(map[string]interface{}); ok {
		role, _ := userMsg["role"].(string)
		if role == "" {
			role = "user"
		}
		content, _ := userMsg["content"].(string)
		messages = []interface{}{
			map[string]interface{}{"role": role, "content": content},
		}
	} else {
		messages = []interface{}{}
	}

	isStream, _ := reqBody["stream"].(bool)

	// Construct clean upstream request body
	var newBody []byte
	if isAnthropic {
		anthropicBody := map[string]interface{}{
			"model":      target.model,
			"messages":   messages,
			"max_tokens": 4096,
			"stream":     isStream,
		}
		for _, k := range []string{"temperature", "top_p", "system", "tools", "tool_choice"} {
			if v, ok := reqBody[k]; ok && v != nil {
				anthropicBody[k] = v
			}
		}
		if mt, ok := reqBody["max_tokens"]; ok && mt != nil {
			anthropicBody["max_tokens"] = mt
		}
		newBody, _ = json.Marshal(anthropicBody)
	} else {
		openaiBody := map[string]interface{}{
			"model":    target.model,
			"messages": messages,
			"stream":   isStream,
		}
		for _, k := range []string{"temperature", "top_p", "max_tokens", "max_completion_tokens", "frequency_penalty", "presence_penalty", "stop", "tools", "tool_choice", "response_format", "seed"} {
			if v, ok := reqBody[k]; ok && v != nil {
				openaiBody[k] = v
			}
		}
		// Also inspect OpenWebUI's nested params
		if params, ok := reqBody["params"].(map[string]interface{}); ok {
			for _, k := range []string{"temperature", "top_p", "max_tokens", "frequency_penalty", "presence_penalty", "stop"} {
				if v, ok := params[k]; ok && v != nil {
					if _, set := openaiBody[k]; !set {
						openaiBody[k] = v
					}
				}
			}
		}
		newBody, _ = json.Marshal(openaiBody)
	}

	req, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(newBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if isStream {
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Accept", "text/event-stream")
	}
	if isAnthropic {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		if target.conn != nil {
			return fmt.Errorf("rate_limited:%d", target.conn.ID)
		}
		return fmt.Errorf("rate_limited")
	}

	if resp.StatusCode == http.StatusNotFound {
		lowerM := strings.ToLower(target.model)
		if strings.Contains(lowerM, "embed") || strings.Contains(lowerM, "retriever") || strings.Contains(lowerM, "rerank") {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Model '%s' is an embedding/retrieval model, not a chat completions model. Upstream returned 404. Please select an instruct/chat model (such as meta/llama-3.2-11b-vision-instruct, mistralai/mistral-large-2-instruct, or nvidia/llama-3.1-nemotron-70b-instruct).", target.model))
			return nil
		}
	}

	// Copy headers excluding hop-by-hop headers
	for k, vv := range resp.Header {
		if hopByHopHeaders[strings.ToLower(k)] {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("X-Router-Provider", target.provider)
	w.Header().Set("X-Router-Model", target.model)

	if isStream && resp.StatusCode == http.StatusOK {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
	}

	w.WriteHeader(resp.StatusCode)

	if isStream && resp.StatusCode == http.StatusOK {
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

	sessionID, _ := reqBody["session_id"].(string)
	hasOpenWebUI := sessionID != "" || reqBody["user_message"] != nil || reqBody["message_ids"] != nil
	if hasOpenWebUI {
		h.handleOpenWebUIChatCompletion(w, r, subdomain, requestedModel, reqBody, bodyBytes)
		return
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
			if err != nil {
				if target.conn != nil {
					excludeConnIDs = append(excludeConnIDs, target.conn.ID)
				}
				if strings.HasPrefix(err.Error(), "rate_limited:") {
					idStr := strings.TrimPrefix(err.Error(), "rate_limited:")
					if id, e := strconv.ParseInt(idStr, 10, 64); e == nil {
						h.db.MarkAIRouterConnectionRateLimited(id)
					}
					lastErr = "connection rate limited"
				} else {
					lastErr = err.Error()
				}
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

// ── OpenWebUI Real-Time WebSocket Streaming & Chat Management ───────────────────

func generateQuickTitle(content string) string {
	s := strings.TrimSpace(content)
	if s == "" {
		return "New Chat"
	}
	lower := strings.ToLower(s)
	for _, p := range []string{"please ", "can you ", "could you ", "help me ", "tolong ", "bantu saya ", "buatkan "} {
		if strings.HasPrefix(lower, p) {
			s = s[len(p):]
			break
		}
	}
	words := strings.Fields(s)
	if len(words) > 6 {
		words = words[:6]
	}
	res := strings.Join(words, " ")
	if len(res) > 0 {
		res = strings.ToUpper(res[:1]) + res[1:]
	}
	return res
}

func (h *APIHandler) buildOpenWebUIMessages(subdomain, userID, chatID, userMessageID string, userContentObj interface{}, reqBody map[string]interface{}) []map[string]interface{} {
	var messages []map[string]interface{}

	// 1. System prompt from params if any
	systemPrompt := ""
	if params, ok := reqBody["params"].(map[string]interface{}); ok {
		if sys, ok := params["system"].(string); ok && sys != "" {
			systemPrompt = sys
		}
	}
	if systemPrompt != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": systemPrompt})
	}

	// 2. Load conversation history from SQLite DB if available
	if chatID != "" && userMessageID != "" {
		_, chatJSON, _, _, _, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, userID, chatID)
		if err == nil && chatJSON != "" {
			var chatData map[string]interface{}
			if json.Unmarshal([]byte(chatJSON), &chatData) == nil {
				chatObj, _ := chatData["chat"].(map[string]interface{})
				if chatObj == nil {
					chatObj = chatData
				}
				if history, ok := chatObj["history"].(map[string]interface{}); ok {
					if msgMap, ok := history["messages"].(map[string]interface{}); ok {
						var chain []map[string]interface{}
						currID := ""
						if userMsg, ok := msgMap[userMessageID].(map[string]interface{}); ok {
							if pid, ok := userMsg["parentId"].(string); ok {
								currID = pid
							}
						}
						seen := make(map[string]bool)
						for currID != "" && !seen[currID] {
							seen[currID] = true
							m, ok := msgMap[currID].(map[string]interface{})
							if !ok {
								break
							}
							role, _ := m["role"].(string)
							content := m["content"]
							if role != "" && content != nil && content != "" {
								chain = append([]map[string]interface{}{{"role": role, "content": content}}, chain...)
							}
							pid, _ := m["parentId"].(string)
							currID = pid
						}
						messages = append(messages, chain...)
					}
				}
			}
		}
	}

	// 3. Append current user message
	if userContentObj != nil && userContentObj != "" {
		messages = append(messages, map[string]interface{}{"role": "user", "content": userContentObj})
	}

	return messages
}

type mcpToolTarget struct {
	serverURL string
	apiKey    string
	toolName  string
}

type accumulatedToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

func (h *APIHandler) resolveMCPToolsForChat(subdomain string, rawToolIDs []interface{}) (openaiTools []map[string]interface{}, antTools []map[string]interface{}, toolMap map[string]mcpToolTarget) {
	toolMap = make(map[string]mcpToolTarget)
	if len(rawToolIDs) == 0 {
		return
	}

	servers, err := h.db.GetOpenWebUIToolServers(subdomain)
	if err != nil || len(servers) == 0 {
		return
	}

	serverMap := make(map[string]*db.OpenWebUIToolServerDB)
	for i := range servers {
		s := &servers[i]
		serverMap[s.ID] = s
		serverMap["server:mcp:"+s.ID] = s
	}

	for _, tidVal := range rawToolIDs {
		tid, _ := tidVal.(string)
		if tid == "" {
			continue
		}
		cleanID := strings.TrimPrefix(tid, "server:mcp:")
		s := serverMap[cleanID]
		if s == nil {
			s = serverMap[tid]
		}
		if s == nil || s.URL == "" {
			continue
		}

		var cfg map[string]any
		_ = json.Unmarshal([]byte(s.ConfigJSON), &cfg)
		if enabled, ok := cfg["enable"].(bool); ok && !enabled {
			continue
		}

		disabledMap := make(map[string]bool)
		if dList, ok := cfg["disabled_tools"].([]any); ok {
			for _, dt := range dList {
				if dStr, ok := dt.(string); ok {
					disabledMap[dStr] = true
				}
			}
		}

		// Also check function_name_filter_list if present
		if filterStr, ok := cfg["function_name_filter_list"].(string); ok && strings.TrimSpace(filterStr) != "" {
			allowed := strings.Split(filterStr, ",")
			allowedMap := make(map[string]bool)
			for _, a := range allowed {
				allowedMap[strings.TrimSpace(a)] = true
			}
			for k := range allowedMap {
				if !allowedMap[k] {
					disabledMap[k] = true
				}
			}
		}

		// Fetch tool list from MCP
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client := NewMCPClient()
		tools, err := client.ListTools(ctx, s.URL, s.APIKey)
		cancel()

		// Fallback to cached specs in InfoJSON if network list fails
		if err != nil || len(tools) == 0 {
			var info map[string]any
			_ = json.Unmarshal([]byte(s.InfoJSON), &info)
			if specs, ok := info["specs"].([]any); ok {
				for _, sp := range specs {
					if spMap, ok := sp.(map[string]any); ok {
						tools = append(tools, spMap)
					}
				}
			}
		}

		for _, t := range tools {
			name, _ := t["name"].(string)
			if name == "" || disabledMap[name] {
				continue
			}
			desc, _ := t["description"].(string)
			schema, _ := t["inputSchema"].(map[string]any)
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}

			toolMap[name] = mcpToolTarget{
				serverURL: s.URL,
				apiKey:    s.APIKey,
				toolName:  name,
			}

			openaiTools = append(openaiTools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        name,
					"description": desc,
					"parameters":  schema,
				},
			})

			antTools = append(antTools, map[string]interface{}{
				"name":         name,
				"description":  desc,
				"input_schema": schema,
			})
		}
	}
	return
}

func (h *APIHandler) streamFollowUpTurn(
	upstreamURL, apiKey, model string,
	isAnthropic bool,
	messages []map[string]interface{},
	accumulatedContent *strings.Builder,
	emitEvent func(string, interface{}),
) error {
	var payloadBytes []byte
	if isAnthropic {
		var antMessages []map[string]interface{}
		sysPrompt := ""
		for _, m := range messages {
			if m["role"] == "system" {
				if s, ok := m["content"].(string); ok {
					sysPrompt = s
				}
			} else {
				antMessages = append(antMessages, m)
			}
		}
		antBody := map[string]interface{}{
			"model":      model,
			"messages":   antMessages,
			"max_tokens": 4096,
			"stream":     true,
		}
		if sysPrompt != "" {
			antBody["system"] = sysPrompt
		}
		payloadBytes, _ = json.Marshal(antBody)
	} else {
		openaiBody := map[string]interface{}{
			"model":    model,
			"messages": messages,
			"stream":   true,
		}
		payloadBytes, _ = json.Marshal(openaiBody)
	}

	req, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Accept-Encoding", "identity")

	if isAnthropic {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{
		Timeout:   300 * time.Second,
		Transport: &http.Transport{DisableCompression: true},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("follow-up upstream error (%d): %s", resp.StatusCode, string(errBytes))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, rErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			if dataStr == "[DONE]" {
				break
			}
			if isAnthropic {
				var antChunk struct {
					Type  string `json:"type"`
					Delta struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"delta"`
				}
				if json.Unmarshal([]byte(dataStr), &antChunk) == nil && antChunk.Delta.Text != "" {
					accumulatedContent.WriteString(antChunk.Delta.Text)
					emitEvent("chat:completion", map[string]interface{}{
						"choices": []interface{}{
							map[string]interface{}{
								"delta": map[string]interface{}{
									"content": antChunk.Delta.Text,
								},
							},
						},
						"done": false,
					})
				}
			} else {
				var oaiChunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(dataStr), &oaiChunk) == nil && len(oaiChunk.Choices) > 0 {
					text := oaiChunk.Choices[0].Delta.Content
					if text != "" {
						accumulatedContent.WriteString(text)
						emitEvent("chat:completion", map[string]interface{}{
							"choices": []interface{}{
								map[string]interface{}{
									"delta": map[string]interface{}{
										"content": text,
									},
								},
							},
							"done": false,
						})
					}
				}
			}
		}
		if rErr != nil {
			break
		}
	}
	return nil
}

func (h *APIHandler) streamTargetToSocket(
	subdomain, userID, chatID, assistantMessageID, folderID string,
	target proxyTarget,
	messages []map[string]interface{},
	reqBody map[string]interface{},
	emitEvent func(string, interface{}),
) error {
	apiType := ""
	baseURL := ""
	apiKey := ""
	if target.conn != nil {
		apiType = target.conn.APIType
		baseURL = target.conn.BaseURL
		apiKey = target.conn.APIKey
	}
	upstreamURL := buildUpstreamURL(target.provider, apiType, baseURL, "openai")
	isAnthropic := isAnthropicTarget(target)

	// Resolve MCP tools from reqBody tool_ids
	var toolIDs []interface{}
	if tList, ok := reqBody["tool_ids"].([]interface{}); ok {
		toolIDs = tList
	}
	oaiTools, antTools, mcpToolMap := h.resolveMCPToolsForChat(subdomain, toolIDs)

	var payloadBytes []byte
	if isAnthropic {
		var antMessages []map[string]interface{}
		sysPrompt := ""
		for _, m := range messages {
			if m["role"] == "system" {
				if s, ok := m["content"].(string); ok {
					sysPrompt = s
				}
			} else {
				antMessages = append(antMessages, m)
			}
		}
		antBody := map[string]interface{}{
			"model":      target.model,
			"messages":   antMessages,
			"max_tokens": 4096,
			"stream":     true,
		}
		if sysPrompt != "" {
			antBody["system"] = sysPrompt
		}
		if len(antTools) > 0 {
			antBody["tools"] = antTools
		}
		payloadBytes, _ = json.Marshal(antBody)
	} else {
		openaiBody := map[string]interface{}{
			"model":    target.model,
			"messages": messages,
			"stream":   true,
		}
		if len(oaiTools) > 0 {
			openaiBody["tools"] = oaiTools
		}
		if params, ok := reqBody["params"].(map[string]interface{}); ok {
			for _, k := range []string{"temperature", "top_p", "max_tokens", "frequency_penalty", "presence_penalty"} {
				if v, ok := params[k]; ok && v != nil {
					openaiBody[k] = v
				}
			}
		}
		payloadBytes, _ = json.Marshal(openaiBody)
	}

	req, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Accept-Encoding", "identity")

	if isAnthropic {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream error (%d): %s", resp.StatusCode, string(errBytes))
	}

	reader := bufio.NewReader(resp.Body)
	var accumulatedContent strings.Builder
	inReasoning := false
	pendingToolCalls := make(map[int]*accumulatedToolCall)

	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line != "" {
			if strings.HasPrefix(line, "data: ") {
				dataStr := strings.TrimPrefix(line, "data: ")
				if dataStr == "[DONE]" {
					break
				}

				if isAnthropic {
					var antChunk struct {
						Type  string `json:"type"`
						Delta struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"delta"`
					}
					if json.Unmarshal([]byte(dataStr), &antChunk) == nil {
						if antChunk.Delta.Text != "" {
							accumulatedContent.WriteString(antChunk.Delta.Text)
							emitEvent("chat:completion", map[string]interface{}{
								"choices": []interface{}{
									map[string]interface{}{
										"delta": map[string]interface{}{
											"content": antChunk.Delta.Text,
										},
									},
								},
								"done": false,
							})
						}
					}
				} else {
					var oaiChunk struct {
						Choices []struct {
							Delta struct {
								Content          string `json:"content"`
								ReasoningContent string `json:"reasoning_content"`
								ToolCalls        []struct {
									Index    int    `json:"index"`
									ID       string `json:"id"`
									Type     string `json:"type"`
									Function struct {
										Name      string `json:"name"`
										Arguments string `json:"arguments"`
									} `json:"function"`
								} `json:"tool_calls"`
							} `json:"delta"`
						} `json:"choices"`
					}
					if json.Unmarshal([]byte(dataStr), &oaiChunk) == nil && len(oaiChunk.Choices) > 0 {
						delta := oaiChunk.Choices[0].Delta
						chunkText := ""

						if delta.ReasoningContent != "" {
							if !inReasoning {
								inReasoning = true
								chunkText = "<think>\n" + delta.ReasoningContent
							} else {
								chunkText = delta.ReasoningContent
							}
						} else if delta.Content != "" {
							if inReasoning {
								inReasoning = false
								chunkText = "\n</think>\n\n" + delta.Content
							} else {
								chunkText = delta.Content
							}
						}

						if chunkText != "" {
							accumulatedContent.WriteString(chunkText)
							emitEvent("chat:completion", map[string]interface{}{
								"choices": []interface{}{
									map[string]interface{}{
										"delta": map[string]interface{}{
											"content": chunkText,
										},
									},
								},
								"done": false,
							})
						}

						// Accumulate any streamed tool calls
						if len(delta.ToolCalls) > 0 {
							for _, tc := range delta.ToolCalls {
								call, exists := pendingToolCalls[tc.Index]
								if !exists {
									call = &accumulatedToolCall{
										id:   tc.ID,
										name: tc.Function.Name,
									}
									pendingToolCalls[tc.Index] = call
								}
								if tc.ID != "" {
									call.id = tc.ID
								}
								if tc.Function.Name != "" {
									call.name = tc.Function.Name
								}
								call.arguments.WriteString(tc.Function.Arguments)
							}
						}
					}
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	if inReasoning {
		accumulatedContent.WriteString("\n</think>\n")
		emitEvent("chat:completion", map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"delta": map[string]interface{}{
						"content": "\n</think>\n",
					},
				},
			},
			"done": false,
		})
	}

	// If the model invoked tool calls, execute them and run follow-up synthesis
	if len(pendingToolCalls) > 0 && len(mcpToolMap) > 0 {
		var executedToolMessages []map[string]interface{}
		var assistantToolCalls []map[string]interface{}

		for idx := 0; idx < len(pendingToolCalls); idx++ {
			tc, ok := pendingToolCalls[idx]
			if !ok {
				continue
			}
			target, exists := mcpToolMap[tc.name]
			if !exists {
				continue
			}

			// Announce tool execution to chat UI
			notice := fmt.Sprintf("\n> ⚡ *Menjalankan tool `%s`...*\n\n", tc.name)
			accumulatedContent.WriteString(notice)
			emitEvent("chat:completion", map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]interface{}{
							"content": notice,
						},
					},
				},
				"done": false,
			})

			var argsObj any
			_ = json.Unmarshal([]byte(tc.arguments.String()), &argsObj)

			execCtx, execCancel := context.WithTimeout(context.Background(), 60*time.Second)
			mcpClient := NewMCPClient()
			rawRes, tErr := mcpClient.ExecuteTool(execCtx, target.serverURL, target.apiKey, target.toolName, argsObj)
			execCancel()

			resBytes, _ := json.Marshal(rawRes)
			resStr := string(resBytes)
			if tErr != nil {
				resStr = "Error: " + tErr.Error()
			}

			assistantToolCalls = append(assistantToolCalls, map[string]interface{}{
				"id":   tc.id,
				"type": "function",
				"function": map[string]interface{}{
					"name":      tc.name,
					"arguments": tc.arguments.String(),
				},
			})

			executedToolMessages = append(executedToolMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tc.id,
				"content":      resStr,
			})
		}

		if len(executedToolMessages) > 0 {
			followUpMessages := append(messages, map[string]interface{}{
				"role":       "assistant",
				"tool_calls": assistantToolCalls,
			})
			followUpMessages = append(followUpMessages, executedToolMessages...)

			_ = h.streamFollowUpTurn(upstreamURL, apiKey, target.model, isAnthropic, followUpMessages, &accumulatedContent, emitEvent)
		}
	}

	// 1. Emit completion
	emitEvent("chat:completion", map[string]interface{}{
		"done": true,
	})

	// 2. Persist assistant message in DB
	finalContent := accumulatedContent.String()
	_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, assistantMessageID, func(msg map[string]interface{}) map[string]interface{} {
		msg["content"] = finalContent
		msg["done"] = true
		return msg
	})

	// 3. Emit chat:active = false
	emitEvent("chat:active", map[string]interface{}{
		"active":    false,
		"folder_id": folderID,
	})

	return nil
}

func (h *APIHandler) handleOpenWebUIChatCompletion(w http.ResponseWriter, r *http.Request, subdomain, requestedModel string, reqBody map[string]interface{}, bodyBytes []byte) {
	u := h.resolveUser(r)
	if u == nil {
		u = &OpenWebUISessionUserInfoResponse{ID: "usr_guest", Name: "Guest", Role: "guest"}
	}

	sessionID, _ := reqBody["session_id"].(string)
	chatID, _ := reqBody["chat_id"].(string)
	if chatID == "" {
		chatID = uuid.New().String()
	}

	assistantMessageID := ""
	if mList, ok := reqBody["message_ids"].([]interface{}); ok && len(mList) > 0 {
		if m0, ok := mList[0].(map[string]interface{}); ok {
			if mid, ok := m0["message_id"].(string); ok && mid != "" {
				assistantMessageID = mid
			}
		}
	}
	if assistantMessageID == "" {
		if idStr, ok := reqBody["id"].(string); ok && idStr != "" {
			assistantMessageID = idStr
		} else {
			assistantMessageID = uuid.New().String()
		}
	}

	var userMessage map[string]interface{}
	if um, ok := reqBody["user_message"].(map[string]interface{}); ok {
		userMessage = um
	}
	userMessageID := ""
	userContent := ""
	var userContentObj interface{}
	if userMessage != nil {
		userMessageID, _ = userMessage["id"].(string)
		userContentObj = userMessage["content"]
		if s, ok := userMessage["content"].(string); ok {
			userContent = s
		}
	}
	folderID, _ := reqBody["folder_id"].(string)

	// Persist initial chat and message state in database
	_, existingChatJSON, _, _, _, _, _, getErr := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, chatID)
	if getErr != nil || existingChatJSON == "" {
		title := "New Chat"
		cleanText := strings.TrimSpace(userContent)
		if cleanText != "" {
			words := strings.Fields(cleanText)
			if len(words) > 6 {
				title = strings.Join(words[:6], " ") + "..."
			} else {
				title = cleanText
			}
		}

		historyMessages := make(map[string]interface{})
		if userMessage != nil && userMessageID != "" {
			userMessage["childrenIds"] = []interface{}{assistantMessageID}
			historyMessages[userMessageID] = userMessage
		}
		historyMessages[assistantMessageID] = map[string]interface{}{
			"id":          assistantMessageID,
			"parentId":    userMessageID,
			"childrenIds": []interface{}{},
			"role":        "assistant",
			"content":     "",
			"done":        false,
			"model":       requestedModel,
			"timestamp":   time.Now().Unix(),
		}

		initialChatObj := map[string]interface{}{
			"id":     chatID,
			"title":  title,
			"models": []interface{}{requestedModel},
			"history": map[string]interface{}{
				"currentId": assistantMessageID,
				"messages":  historyMessages,
			},
			"messages": []interface{}{
				map[string]interface{}{"role": "user", "content": userContent},
			},
			"timestamp": time.Now().Unix() * 1000,
		}
		chatBytes, _ := json.Marshal(map[string]interface{}{"chat": initialChatObj})
		_ = h.db.UpsertOpenWebUIChat(subdomain, u.ID, chatID, title, string(chatBytes), folderID)
	} else {
		_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, u.ID, chatID, assistantMessageID, func(msg map[string]interface{}) map[string]interface{} {
			msg["id"] = assistantMessageID
			if userMessageID != "" {
				msg["parentId"] = userMessageID
			}
			msg["role"] = "assistant"
			msg["content"] = ""
			msg["done"] = false
			msg["model"] = requestedModel
			msg["timestamp"] = time.Now().Unix()
			return msg
		})
		if userMessage != nil && userMessageID != "" {
			_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, u.ID, chatID, userMessageID, func(msg map[string]interface{}) map[string]interface{} {
				for k, v := range userMessage {
					msg[k] = v
				}
				cIds, _ := msg["childrenIds"].([]interface{})
				found := false
				for _, cid := range cIds {
					if cid == assistantMessageID {
						found = true
						break
					}
				}
				if !found {
					msg["childrenIds"] = append(cIds, assistantMessageID)
				}
				return msg
			})
		}
	}

	emitEvent := func(eventType string, eventData interface{}) {
		payload := map[string]interface{}{
			"chat_id":    chatID,
			"message_id": assistantMessageID,
			"data": map[string]interface{}{
				"type": eventType,
				"data": eventData,
			},
		}
		msgBytes, err := json.Marshal(payload)
		if err != nil {
			return
		}
		socketMsg := []byte(fmt.Sprintf(`42["events",%s]`, string(msgBytes)))

		sentDirectly := false
		if sessionID != "" {
			if client := globalSocketHub.getClient(sessionID); client != nil {
				_ = client.write(socketMsg)
				sentDirectly = true
			}
		}
		if u != nil && u.ID != "" {
			skipSid := ""
			if sentDirectly {
				skipSid = sessionID
			}
			globalSocketHub.broadcastToRoom("user:"+u.ID, socketMsg, skipSid)
		}
	}

	taskID := uuid.New().String()

	// Return immediate JSON response expected by OpenWebUI frontend
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   true,
		"task_ids": []string{taskID},
		"chat_id":  chatID,
	})

	// Run streaming in the background
	go func() {
		emitEvent("chat:active", map[string]interface{}{
			"active":    true,
			"folder_id": folderID,
		})

		messages := h.buildOpenWebUIMessages(subdomain, u.ID, chatID, userMessageID, userContentObj, reqBody)

		targets, err := h.resolveTargets(subdomain, requestedModel, "/api/chat/completions", bodyBytes, nil)
		if err != nil || len(targets) == 0 {
			errMsg := "No active connection for model: " + requestedModel
			if err != nil {
				errMsg = err.Error()
			}
			emitEvent("chat:message:error", map[string]interface{}{
				"error": map[string]interface{}{"content": errMsg},
				"done":  true,
			})
			emitEvent("chat:active", map[string]interface{}{"active": false, "folder_id": folderID})
			return
		}

		var lastErr error
		for _, target := range targets {
			lastErr = h.streamTargetToSocket(subdomain, u.ID, chatID, assistantMessageID, folderID, target, messages, reqBody, emitEvent)
			if lastErr == nil {
				break
			}
		}

		if lastErr != nil {
			emitEvent("chat:message:error", map[string]interface{}{
				"error": map[string]interface{}{"content": lastErr.Error()},
				"done":  true,
			})
			emitEvent("chat:active", map[string]interface{}{"active": false, "folder_id": folderID})
			return
		}

		// Title generation if requested by background_tasks
		bgTasks, _ := reqBody["background_tasks"].(map[string]interface{})
		if bgTasks != nil {
			if tg, ok := bgTasks["title_generation"].(bool); ok && tg {
				if userContent != "" {
					generatedTitle := generateQuickTitle(userContent)
					if generatedTitle != "" {
						_ = h.db.UpdateOpenWebUIChatTitle(subdomain, u.ID, chatID, generatedTitle)
						emitEvent("chat:title", generatedTitle)
					}
				}
			}
		}
	}()
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
