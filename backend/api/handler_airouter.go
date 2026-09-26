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
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	case strings.Contains(s, "gemini-1.5-pro") || strings.Contains(s, "gemini-2.0-pro"):
		return 2000000
	case strings.Contains(s, "gemini-1.5") || strings.Contains(s, "gemini-2.0") || strings.Contains(s, "gemini-flash"):
		return 1000000
	case strings.Contains(s, "claude-3-7") || strings.Contains(s, "claude-3-5") || strings.Contains(s, "claude-3"):
		return 200000
	case strings.Contains(s, "gpt-4o") || strings.Contains(s, "o1") || strings.Contains(s, "o3") || strings.Contains(s, "gpt-4.5") || strings.Contains(s, "gpt-4-turbo"):
		return 128000
	case strings.Contains(s, "deepseek-r1") || strings.Contains(s, "deepseek-v3") || strings.Contains(s, "deepseek-chat") || strings.Contains(s, "deepseek-coder"):
		return 128000
	case strings.Contains(s, "qwen-2.5") || strings.Contains(s, "qwen2.5"):
		return 128000
	case strings.Contains(s, "llama-3.1") || strings.Contains(s, "llama-3.3") || strings.Contains(s, "llama-3.2"):
		return 128000
	case strings.Contains(s, "mistral-large") || strings.Contains(s, "mistral-small") || strings.Contains(s, "mistral-nemo") || strings.Contains(s, "pixtral"):
		return 128000
	case strings.Contains(s, "phi-4"):
		return 128000
	case strings.Contains(s, "codestral") || strings.Contains(s, "mixtral"):
		return 32768
	case strings.Contains(s, "gpt-4-32k"):
		return 32768
	case strings.Contains(s, "gpt-3.5-turbo-16k"):
		return 16384
	case strings.Contains(s, "gpt-4") || strings.Contains(s, "llama-3-") || strings.Contains(s, "gemma-2") || strings.Contains(s, "gemma2"):
		return 8192
	case strings.Contains(s, "gpt-3.5") || strings.Contains(s, "phi-3"):
		return 4096
	default:
		return 128000
	}
}

// getModelContextSize resolves context size from DB mapping -> alias target -> auto-detection
func (h *APIHandler) getModelContextSize(subdomain, modelName string) int {
	if mc, err := h.db.GetAIRouterModelContext(subdomain, modelName); err == nil && mc != nil && mc.ContextSize > 0 {
		return mc.ContextSize
	}
	resolved := h.db.ResolveAIRouterAlias(subdomain, modelName)
	if resolved != modelName {
		if mc, err := h.db.GetAIRouterModelContext(subdomain, resolved); err == nil && mc != nil && mc.ContextSize > 0 {
			return mc.ContextSize
		}
	}
	return getDefaultContextSize(resolved)
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
	case "antigravity":
		return "https://daily-cloudcode-pa.googleapis.com"
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

// ── Antigravity OAuth & Quota Constants ─────────────────────────────────────

func decodeSecret(bytes []byte, key byte) string {
	res := make([]byte, len(bytes))
	for i, b := range bytes {
		res[i] = b ^ key
	}
	return string(res)
}

func getAntigravityClientID() string {
	if v := os.Getenv("ANTIGRAVITY_CLIENT_ID"); v != "" {
		return v
	}
	raw := []byte{107, 106, 109, 107, 106, 106, 108, 106, 108, 106, 111, 99, 107, 119, 46, 55, 50, 41, 41, 51, 52, 104, 50, 104, 107, 54, 57, 40, 63, 104, 105, 111, 44, 46, 53, 54, 53, 48, 50, 110, 61, 110, 106, 105, 63, 42, 116, 59, 42, 42, 41, 116, 61, 53, 53, 61, 54, 63, 47, 41, 63, 40, 57, 53, 52, 46, 63, 52, 46, 116, 57, 53, 55}
	return decodeSecret(raw, 0x5a)
}

func getAntigravityClientSecret() string {
	if v := os.Getenv("ANTIGRAVITY_CLIENT_SECRET"); v != "" {
		return v
	}
	raw := []byte{29, 21, 25, 9, 10, 2, 119, 17, 111, 98, 28, 13, 8, 110, 98, 108, 22, 62, 22, 16, 107, 55, 22, 24, 98, 41, 2, 25, 110, 32, 108, 43, 30, 27, 60}
	return decodeSecret(raw, 0x5a)
}

const (
	antigravityAuthorizeURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	antigravityTokenURL          = "https://oauth2.googleapis.com/token"
	antigravityUserInfoURL       = "https://www.googleapis.com/oauth2/v1/userinfo"
	antigravityBaseURL           = "https://daily-cloudcode-pa.googleapis.com"
	antigravityUserAgent         = "antigravity/ide/2.11.0 darwin/arm64"
	antigravityLoadCodeAssistURL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	antigravityOnboardUserURL    = "https://cloudcode-pa.googleapis.com/v1internal:onboardUser"
)

var defaultAntigravityModels = []string{
	// Claude models (always have quota, not shared with Gemini pools)
	"claude-sonnet-4-6",
	"claude-opus-4-6-thinking",
	// Gemini 3.8 Flash (3 quota tiers + alias)
	"gemini-3.8-flash-high",
	"gemini-3.8-flash-medium",
	"gemini-3.8-flash-low",
	"gemini-3.8-flash", // alias → medium tier
	// Gemini 3.7 Flash
	"gemini-3.7-flash-high",
	"gemini-3.7-flash-medium",
	"gemini-3.7-flash-low",
	// Gemini 3.6 Flash
	"gemini-3.6-flash-high",
	"gemini-3.6-flash-medium",
	"gemini-3.6-flash-low",
	// Gemini 3.5 Flash
	"gemini-3.5-flash-high",
	"gemini-3.5-flash-low",
	"gemini-3.5-flash-extra-low",
	"gemini-3-flash-agent",
	// Gemini Pro
	"gemini-pro-agent",    // Gemini 3.1 Pro (High)
	"gemini-3.1-pro-low",  // Gemini 3.1 Pro (Low)
	// GPT-OSS
	"gpt-oss-120b-medium",
}

type antigravityQuotaCacheEntry struct {
	remainingFraction float64
	resetAt           string
	cachedAt          time.Time
}

var (
	agQuotaCache   = make(map[string]antigravityQuotaCacheEntry)
	agQuotaCacheMu sync.RWMutex
)

// ensureFreshAntigravityToken checks token validity and auto-refreshes using refreshToken if expiring
func (h *APIHandler) ensureFreshAntigravityToken(c *db.AIRouterConnection) error {
	if c == nil || c.Provider != "antigravity" || c.RefreshToken == "" {
		return nil
	}
	if c.TokenExpiresAt != nil && time.Until(*c.TokenExpiresAt) > 5*time.Minute {
		return nil
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", c.RefreshToken)
	data.Set("client_id", getAntigravityClientID())
	data.Set("client_secret", getAntigravityClientSecret())

	resp, err := http.Post(antigravityTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(b))
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	c.APIKey = res.AccessToken
	newExpiry := time.Now().Add(time.Duration(res.ExpiresIn) * time.Second)
	c.TokenExpiresAt = &newExpiry

	_ = h.db.UpdateAIRouterConnectionToken(c.ID, c.APIKey, c.RefreshToken, c.TokenExpiresAt)
	return nil
}

// getAntigravityModelQuota checks live model quota with 60s in-memory caching
func (h *APIHandler) getAntigravityModelQuota(c *db.AIRouterConnection, model string) float64 {
	key := fmt.Sprintf("%d:%s", c.ID, model)
	agQuotaCacheMu.RLock()
	entry, found := agQuotaCache[key]
	agQuotaCacheMu.RUnlock()

	if found && time.Since(entry.cachedAt) < 60*time.Second {
		return entry.remainingFraction
	}

	_ = h.ensureFreshAntigravityToken(c)
	bodyMap := map[string]interface{}{}
	if c.ProjectID != "" {
		bodyMap["project"] = c.ProjectID
	}
	reqBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest(http.MethodPost, antigravityBaseURL+"/v1internal:fetchAvailableModels", bytes.NewReader(reqBytes))
	if err != nil {
		return 1.0
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("User-Agent", antigravityUserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Name", "antigravity")
	req.Header.Set("X-Client-Version", "2.11.0")

	client, _ := createProxyHTTPClient(c.Socks5Proxy, 10*time.Second)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return 1.0
	}
	defer resp.Body.Close()

	var data struct {
		Models map[string]struct {
			QuotaInfo *struct {
				RemainingFraction float64 `json:"remainingFraction"`
				ResetTime         string  `json:"resetTime"`
			} `json:"quotaInfo"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && data.Models != nil {
		agQuotaCacheMu.Lock()
		for mKey, mInfo := range data.Models {
			if mInfo.QuotaInfo != nil {
				agQuotaCache[fmt.Sprintf("%d:%s", c.ID, mKey)] = antigravityQuotaCacheEntry{
					remainingFraction: mInfo.QuotaInfo.RemainingFraction,
					resetAt:           mInfo.QuotaInfo.ResetTime,
					cachedAt:          time.Now(),
				}
			}
		}
		agQuotaCacheMu.Unlock()
	}

	agQuotaCacheMu.RLock()
	entry, found = agQuotaCache[key]
	agQuotaCacheMu.RUnlock()
	if found {
		return entry.remainingFraction
	}
	return 1.0
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

		case "usage":
			// Multi-account Antigravity Quota Routing:
			// Balance across connected Antigravity accounts by remaining quota & failover on 429
			h.db.ReactivateExpiredAIRouterConnections()
			activeConns, err := h.db.GetActiveAIRouterConnectionsUnmasked(subdomain, excludeConnIDs)
			if err != nil || len(activeConns) == 0 {
				return nil, fmt.Errorf("no active connections available for combo '%s'", combo.Name)
			}

			var agConns []db.AIRouterConnection
			for _, c := range activeConns {
				if c.Provider == "antigravity" {
					agConns = append(agConns, c)
				}
			}

			if len(agConns) == 0 {
				// Fallback to round-robin if no Antigravity accounts connected
				idx := rrNext(subdomain+"/"+combo.Name, len(models))
				return h.resolveTargets(subdomain, models[idx], endpoint, bodyBytes, excludeConnIDs)
			}

			targetModel := models[0]
			cleanModel := strings.TrimPrefix(targetModel, "antigravity/")

			type rankedConn struct {
				conn     db.AIRouterConnection
				fraction float64
			}
			var ranked []rankedConn
			for _, c := range agConns {
				cCopy := c
				f := h.getAntigravityModelQuota(&cCopy, cleanModel)
				ranked = append(ranked, rankedConn{conn: cCopy, fraction: f})
			}

			// Sort descending by remaining quota
			sort.Slice(ranked, func(i, j int) bool {
				if ranked[i].fraction != ranked[j].fraction {
					return ranked[i].fraction > ranked[j].fraction
				}
				return ranked[i].conn.ID < ranked[j].conn.ID
			})

			// Group top tied accounts and rotate among them
			topFraction := ranked[0].fraction
			var topTier []rankedConn
			var lowerTier []rankedConn
			for _, rk := range ranked {
				if rk.fraction >= topFraction-0.05 {
					topTier = append(topTier, rk)
				} else {
					lowerTier = append(lowerTier, rk)
				}
			}
			if len(topTier) > 1 {
				offset := rrNext(subdomain+"/"+combo.Name+"/usage", len(topTier))
				rotated := make([]rankedConn, len(topTier))
				for i := range topTier {
					rotated[i] = topTier[(i+offset)%len(topTier)]
				}
				topTier = rotated
			}

			var targets []proxyTarget
			for _, rk := range append(topTier, lowerTier...) {
				connCopy := rk.conn
				targets = append(targets, proxyTarget{
					conn:     &connCopy,
					model:    targetModel,
					provider: "antigravity",
				})
			}
			return targets, nil

		case "smart_context":
			tokens := estimateInputTokens(bodyBytes)
			type candidate struct {
				model   string
				ctxSize int
			}
			var candidates []candidate
			for _, m := range models {
				ctx := h.getModelContextSize(subdomain, m)
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
		standardProviders := []string{"openai", "anthropic", "antigravity", "deepseek", "groq", "together", "openrouter", "custom"}
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
	case strings.Contains(s, "antigravity"), strings.Contains(s, "sonnet-4"), strings.Contains(s, "opus-4"), strings.Contains(s, "gemini-3"), strings.Contains(s, "gpt-oss"):
		preferredProviders = []string{"antigravity", "anthropic", "openrouter", "custom"}
	case strings.Contains(s, "claude"):
		preferredProviders = []string{"antigravity", "anthropic", "openrouter", "custom"}
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

func (h *APIHandler) forwardRequest(w http.ResponseWriter, bodyBytes []byte, target proxyTarget, inputFormat, outputFormat string) error {
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

	if target.conn != nil && target.conn.Provider == "antigravity" {
		return h.forwardAntigravityRequest(w, bodyBytes, target, isStream)
	}

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

	proxyAddr := ""
	if target.conn != nil {
		proxyAddr = target.conn.Socks5Proxy
	}
	client, cErr := createProxyHTTPClient(proxyAddr, 60*time.Second)
	if cErr != nil {
		return fmt.Errorf("proxy error: %w", cErr)
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
			err := h.forwardRequest(w, bodyBytes, target, inputFormat, outputFormat)
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
	loadedFromDB := false
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
						if len(chain) > 0 {
							messages = append(messages, chain...)
							loadedFromDB = true
						}
					}
				}
			}
		}
	}

	// 3. Fallback to client-provided messages in reqBody if DB chain is not found or empty (e.g. continuing an assistant message)
	if !loadedFromDB {
		if rawMsgs, ok := reqBody["messages"].([]interface{}); ok && len(rawMsgs) > 0 {
			for _, item := range rawMsgs {
				if mMap, ok := item.(map[string]interface{}); ok {
					r, _ := mMap["role"].(string)
					c := mMap["content"]
					if r != "" && c != nil {
						if r == "system" && systemPrompt != "" {
							continue
						}
						messages = append(messages, map[string]interface{}{"role": r, "content": c})
					}
				}
			}
			return messages
		}
	}

	// 4. Append current user message
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

	// 1. Register OpenWebUI builtin tools
	builtinSpecs := GetBuiltinToolSpecs()
	for _, spec := range builtinSpecs {
		fnName, _ := spec.Function["name"].(string)
		desc, _ := spec.Function["description"].(string)
		params, _ := spec.Function["parameters"].(map[string]any)

		openaiTools = append(openaiTools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        fnName,
				"description": desc,
				"parameters":  params,
			},
		})

		antTools = append(antTools, map[string]interface{}{
			"name":         fnName,
			"description":  desc,
			"input_schema": params,
		})

		toolMap[fnName] = mcpToolTarget{
			serverURL: "builtin",
			toolName:  fnName,
		}
	}

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
	socks5Proxy string,
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

	client, cErr := createProxyHTTPClient(socks5Proxy, 300*time.Second)
	if cErr != nil {
		return fmt.Errorf("proxy error: %w", cErr)
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
	thinkFilter := newThinkingStreamFilter()
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
						Type     string `json:"type"`
						Text     string `json:"text"`
						Thinking string `json:"thinking"`
					} `json:"delta"`
				}
				if json.Unmarshal([]byte(dataStr), &antChunk) == nil {
					chunkText := ""
					if antChunk.Delta.Thinking != "" {
						chunkText = thinkFilter.ProcessReasoning(antChunk.Delta.Thinking)
					} else if antChunk.Delta.Text != "" {
						chunkText = thinkFilter.ProcessContent(antChunk.Delta.Text)
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
				}
			} else {
				var oaiChunk struct {
					Choices []struct {
						Delta struct {
							Content          string `json:"content"`
							ReasoningContent string `json:"reasoning_content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(dataStr), &oaiChunk) == nil && len(oaiChunk.Choices) > 0 {
					delta := oaiChunk.Choices[0].Delta
					chunkText := ""
					if delta.ReasoningContent != "" {
						chunkText = thinkFilter.ProcessReasoning(delta.ReasoningContent)
					} else if delta.Content != "" {
						chunkText = thinkFilter.ProcessContent(delta.Content)
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
				}
			}
		}
		if rErr != nil {
			break
		}
	}
	if flush := thinkFilter.Flush(); flush != "" {
		accumulatedContent.WriteString(flush)
		emitEvent("chat:completion", map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"delta": map[string]interface{}{
						"content": flush,
					},
				},
			},
			"done": false,
		})
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

	if target.conn != nil && target.conn.Provider == "antigravity" {
		return h.streamAntigravityToSocket(subdomain, target, messages, reqBody, emitEvent)
	}

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

	proxyAddr := ""
	if target.conn != nil {
		proxyAddr = target.conn.Socks5Proxy
	}
	client, cErr := createProxyHTTPClient(proxyAddr, 300*time.Second)
	if cErr != nil {
		return fmt.Errorf("proxy error: %w", cErr)
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
	thinkFilter := newThinkingStreamFilter()
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
							Type     string `json:"type"`
							Text     string `json:"text"`
							Thinking string `json:"thinking"`
						} `json:"delta"`
					}
					if json.Unmarshal([]byte(dataStr), &antChunk) == nil {
						chunkText := ""
						if antChunk.Delta.Thinking != "" {
							chunkText = thinkFilter.ProcessReasoning(antChunk.Delta.Thinking)
						} else if antChunk.Delta.Text != "" {
							chunkText = thinkFilter.ProcessContent(antChunk.Delta.Text)
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
							chunkText = thinkFilter.ProcessReasoning(delta.ReasoningContent)
						} else if delta.Content != "" {
							chunkText = thinkFilter.ProcessContent(delta.Content)
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

	if flush := thinkFilter.Flush(); flush != "" {
		accumulatedContent.WriteString(flush)
		emitEvent("chat:completion", map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"delta": map[string]interface{}{
						"content": flush,
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

			var rawRes any
			var tErr error

			if target.serverURL == "builtin" {
				var argsMap map[string]any
				if m, ok := argsObj.(map[string]any); ok {
					argsMap = m
				} else {
					argsMap = make(map[string]any)
				}
				execCtx, execCancel := context.WithTimeout(context.Background(), 60*time.Second)
				rawRes, tErr = h.ExecuteBuiltinTool(execCtx, target.toolName, argsMap, subdomain, userID, chatID)
				execCancel()
			} else {
				execCtx, execCancel := context.WithTimeout(context.Background(), 60*time.Second)
				mcpClient := NewMCPClient()
				rawRes, tErr = mcpClient.ExecuteTool(execCtx, target.serverURL, target.apiKey, target.toolName, argsObj)
				execCancel()
			}

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

			_ = h.streamFollowUpTurn(upstreamURL, apiKey, target.model, isAnthropic, followUpMessages, &accumulatedContent, emitEvent, proxyAddr)
		}
	}

	// 1. Emit completion
	emitEvent("chat:completion", map[string]interface{}{
		"done": true,
	})

	// 2. Persist assistant message in DB
	finalContent := accumulatedContent.String()
	_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, userID, chatID, assistantMessageID, func(msg map[string]interface{}) map[string]interface{} {
		if finalContent != "" {
			msg["content"] = finalContent
		}
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

	var previousContent string
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
			if c, ok := msg["content"].(string); ok && strings.TrimSpace(c) != "" {
				previousContent = c
				msg["originalContent"] = c
			}
			msg["id"] = assistantMessageID
			if userMessageID != "" {
				msg["parentId"] = userMessageID
			}
			msg["role"] = "assistant"
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
			_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, u.ID, chatID, assistantMessageID, func(msg map[string]interface{}) map[string]interface{} {
				msg["done"] = true
				msg["error"] = map[string]interface{}{"content": errMsg}
				if curr, ok := msg["content"].(string); !ok || strings.TrimSpace(curr) == "" {
					if previousContent != "" {
						msg["content"] = previousContent
					} else {
						msg["content"] = fmt.Sprintf("⚠️ *Error: %s*", errMsg)
					}
				}
				return msg
			})
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
			errMsg := lastErr.Error()
			_ = h.db.UpdateOpenWebUIMessageInChat(subdomain, u.ID, chatID, assistantMessageID, func(msg map[string]interface{}) map[string]interface{} {
				msg["done"] = true
				msg["error"] = map[string]interface{}{"content": errMsg}
				if curr, ok := msg["content"].(string); !ok || strings.TrimSpace(curr) == "" {
					if previousContent != "" {
						msg["content"] = previousContent
					} else {
						msg["content"] = fmt.Sprintf("⚠️ *Error: %s*", errMsg)
					}
				}
				return msg
			})
			emitEvent("chat:message:error", map[string]interface{}{
				"error": map[string]interface{}{"content": errMsg},
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
	modelContexts, _ := h.db.GetAIRouterModelContexts(subdomain)
	userKeys, _ := h.db.GetAIRouterUserKeys(subdomain)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":         cfg,
		"connections":    conns,
		"combos":         combos,
		"aliases":        aliases,
		"model_contexts": modelContexts,
		"user_keys":      userKeys,
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
	ID          int64  `json:"id,omitempty"`
	Provider    string `json:"provider"`
	APIType     string `json:"api_type"`
	APIKey      string `json:"api_key"`
	BaseURL     string `json:"base_url"`
	Socks5Proxy string `json:"socks5_proxy,omitempty"`
}

func testProviderConnection(provider, apiType, apiKey, customBaseURL, socks5Proxy string) (bool, int64, []string, string) {
	start := time.Now()
	baseURL := providerBaseURL(provider, customBaseURL)
	isAnthropic := (apiType == "anthropic" || (apiType == "" && provider == "anthropic"))

	client, cErr := createProxyHTTPClient(socks5Proxy, 15*time.Second)
	if cErr != nil {
		return false, time.Since(start).Milliseconds(), nil, fmt.Sprintf("Proxy configuration error: %v", cErr)
	}
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
			if body.Socks5Proxy == "" {
				body.Socks5Proxy = conn.Socks5Proxy
			}
		}
	}

	if body.APIKey == "" {
		writeJSONError(w, http.StatusBadRequest, "API Key is required to test connection")
		return
	}

	success, latency, models, msg := testProviderConnection(body.Provider, body.APIType, body.APIKey, body.BaseURL, body.Socks5Proxy)
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
			if body.Socks5Proxy == "" {
				body.Socks5Proxy = conn.Socks5Proxy
			}
		}
	}

	success, _, models, msg := testProviderConnection(body.Provider, body.APIType, body.APIKey, body.BaseURL, body.Socks5Proxy)
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

// ── Model Context Sizes CRUD ──────────────────────────────────────────────────

func (h *APIHandler) handleGetAIRouterModelContexts(w http.ResponseWriter, r *http.Request, subdomain string) {
	contexts, err := h.db.GetAIRouterModelContexts(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if contexts == nil {
		contexts = []db.AIRouterModelContext{}
	}
	writeJSON(w, http.StatusOK, contexts)
}

func (h *APIHandler) handleUpsertAIRouterModelContext(w http.ResponseWriter, r *http.Request, subdomain string) {
	var body struct {
		ModelName   string `json:"model_name"`
		ContextSize int    `json:"context_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	body.ModelName = strings.TrimSpace(body.ModelName)
	if body.ModelName == "" {
		writeJSONError(w, http.StatusBadRequest, "model_name required")
		return
	}
	if body.ContextSize <= 0 {
		body.ContextSize = getDefaultContextSize(body.ModelName)
	}
	if err := h.db.UpsertAIRouterModelContext(subdomain, body.ModelName, body.ContextSize); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"model_name":   body.ModelName,
		"context_size": body.ContextSize,
	})
}

func (h *APIHandler) handleDeleteAIRouterModelContext(w http.ResponseWriter, r *http.Request, subdomain string) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	_ = h.db.DeleteAIRouterModelContext(subdomain, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleAutoDetectModelContexts(w http.ResponseWriter, r *http.Request, subdomain string) {
	var req struct {
		Models []string `json:"models"`
		Save   bool     `json:"save"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	type detectedItem struct {
		ModelName   string `json:"model_name"`
		ContextSize int    `json:"context_size"`
		IsCustom    bool   `json:"is_custom"`
	}
	var result []detectedItem

	existingMap := make(map[string]int)
	if customList, err := h.db.GetAIRouterModelContexts(subdomain); err == nil {
		for _, c := range customList {
			existingMap[c.ModelName] = c.ContextSize
		}
	}

	for _, m := range req.Models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if customCtx, ok := existingMap[m]; ok && customCtx > 0 {
			result = append(result, detectedItem{ModelName: m, ContextSize: customCtx, IsCustom: true})
		} else {
			ctx := getDefaultContextSize(m)
			if req.Save {
				_ = h.db.UpsertAIRouterModelContext(subdomain, m, ctx)
			}
			result = append(result, detectedItem{ModelName: m, ContextSize: ctx, IsCustom: false})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"detected": result,
	})
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

// handleAIRouterImages handles /v1/images/generations and /v1/images/edits
func (h *APIHandler) handleAIRouterImages(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	subdomain := r.PathValue("subdomain")
	if subdomain == "" {
		subdomain = extractSubdomainFromHost(r.Host)
	}
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
		subdomain = h.resolveSubdomain(r)
	}
	if subdomain == "" {
		writeJSONError(w, http.StatusBadRequest, "could not determine subdomain")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var reqMap map[string]any
	if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed JSON body")
		return
	}

	action := "generate_image"
	if strings.Contains(r.URL.Path, "edits") {
		action = "edit_image"
	}

	prompt, _ := reqMap["prompt"].(string)
	size, _ := reqMap["size"].(string)
	if size == "" {
		size = "1024x1024"
	}
	model, _ := reqMap["model"].(string)
	if model == "" {
		model = "dall-e-3"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	result, err := h.dispatchImageGeneration(ctx, subdomain, action, prompt, size, model, reqMap)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ── Antigravity Request Forwarding & Streaming ──────────────────────────────

func (h *APIHandler) forwardAntigravityRequest(w http.ResponseWriter, bodyBytes []byte, target proxyTarget, isStream bool) error {
	_ = h.ensureFreshAntigravityToken(target.conn)
	apiKey := target.conn.APIKey
	projectID := target.conn.ProjectID

	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)

	var contents []map[string]interface{}
	var systemParts []map[string]interface{}

	if rawMsgs, ok := reqBody["messages"].([]interface{}); ok {
		for _, m := range rawMsgs {
			if mObj, ok := m.(map[string]interface{}); ok {
				role, _ := mObj["role"].(string)
				text := ""
				if cStr, ok := mObj["content"].(string); ok {
					text = cStr
				} else if cArr, ok := mObj["content"].([]interface{}); ok {
					for _, part := range cArr {
						if pMap, ok := part.(map[string]interface{}); ok {
							if pt, ok := pMap["text"].(string); ok {
								text += pt
							}
						}
					}
				}

				if role == "system" {
					systemParts = append(systemParts, map[string]interface{}{"text": text})
				} else {
					geminiRole := "user"
					if role == "assistant" {
						geminiRole = "model"
					}
					contents = append(contents, map[string]interface{}{
						"role":  geminiRole,
						"parts": []map[string]interface{}{{"text": text}},
					})
				}
			}
		}
	}

	if len(contents) == 0 {
		contents = append(contents, map[string]interface{}{
			"role":  "user",
			"parts": []map[string]interface{}{{"text": "Hello"}},
		})
	}

	requestObj := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     1.0,
			"maxOutputTokens": 8192,
		},
	}
	if len(systemParts) > 0 {
		requestObj["systemInstruction"] = map[string]interface{}{
			"role":  "user",
			"parts": systemParts,
		}
	}

	reqID := fmt.Sprintf("agent/%s/%d/%s/1", uuid.New().String(), time.Now().UnixMilli(), uuid.New().String())
	envelope := map[string]interface{}{
		"project":   projectID,
		"model":     target.model,
		"userAgent": "antigravity",
		"requestId": reqID,
		"request":   requestObj,
	}

	envBytes, _ := json.Marshal(envelope)
	action := "generateContent"
	if isStream {
		action = "streamGenerateContent?alt=sse"
	}
	urlStr := fmt.Sprintf("%s/v1internal:%s", antigravityBaseURL, action)

	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(envBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", antigravityUserAgent)
	req.Header.Set("X-Client-Name", "antigravity")
	req.Header.Set("X-Client-Version", "2.11.0")

	proxyAddr := ""
	if target.conn != nil {
		proxyAddr = target.conn.Socks5Proxy
	}
	client, cErr := createProxyHTTPClient(proxyAddr, 120*time.Second)
	if cErr != nil {
		return fmt.Errorf("proxy error: %w", cErr)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Per-model quota: cache quota=0 for this model only, don't block the whole connection.
		agQuotaCacheMu.Lock()
		agQuotaCache[fmt.Sprintf("%d:%s", target.conn.ID, target.model)] = antigravityQuotaCacheEntry{
			remainingFraction: 0.0,
			cachedAt:          time.Now(),
			resetAt:           time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		agQuotaCacheMu.Unlock()
		return fmt.Errorf("rate_limited:%d", target.conn.ID)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		bStr := string(b)
		if strings.Contains(bStr, "RESOURCE_EXHAUSTED") || strings.Contains(bStr, "Quota") {
			// Per-model quota exhausted: cache quota=0 for this model only, not the whole connection.
			agQuotaCacheMu.Lock()
			agQuotaCache[fmt.Sprintf("%d:%s", target.conn.ID, target.model)] = antigravityQuotaCacheEntry{
				remainingFraction: 0.0,
				cachedAt:          time.Now(),
				resetAt:           time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			}
			agQuotaCacheMu.Unlock()
			return fmt.Errorf("rate_limited:%d", target.conn.ID)
		}
		return fmt.Errorf("antigravity upstream error (%d): %s", resp.StatusCode, bStr)
	}

	w.Header().Set("X-Router-Provider", "antigravity")
	w.Header().Set("X-Router-Model", target.model)

	if isStream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		reader := bufio.NewReader(resp.Body)
		flusher, _ := w.(http.Flusher)
		chunkID := "chatcmpl-" + uuid.New().String()

		for {
			line, rErr := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line != "" && strings.HasPrefix(line, "data: ") {
				jsonStr := strings.TrimPrefix(line, "data: ")
				var agResp struct {
					Response struct {
						Candidates []struct {
							Content struct {
								Parts []struct {
									Text    string `json:"text"`
									Thought bool   `json:"thought"`
								} `json:"parts"`
							} `json:"content"`
						} `json:"candidates"`
					} `json:"response"`
				}
				if json.Unmarshal([]byte(jsonStr), &agResp) == nil {
					for _, cand := range agResp.Response.Candidates {
						for _, part := range cand.Content.Parts {
							if part.Text != "" && !part.Thought {
								chunkPayload := map[string]interface{}{
									"id":      chunkID,
									"object":  "chat.completion.chunk",
									"created": time.Now().Unix(),
									"model":   target.model,
									"choices": []interface{}{
										map[string]interface{}{
											"index":         0,
											"delta":         map[string]interface{}{"content": part.Text},
											"finish_reason": nil,
										},
									},
								}
								chunkBytes, _ := json.Marshal(chunkPayload)
								fmt.Fprintf(w, "data: %s\n\n", string(chunkBytes))
								if flusher != nil {
									flusher.Flush()
								}
							}
						}
					}
				}
			}
			if rErr != nil {
				break
			}
		}

		finalPayload := map[string]interface{}{
			"id":      chunkID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   target.model,
			"choices": []interface{}{
				map[string]interface{}{
					"index":         0,
					"delta":         map[string]interface{}{},
					"finish_reason": "stop",
				},
			},
		}
		fBytes, _ := json.Marshal(finalPayload)
		fmt.Fprintf(w, "data: %s\n\n", string(fBytes))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	// Non-streaming
	var agResp struct {
		Response struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agResp); err != nil {
		return err
	}
	var fullText strings.Builder
	for _, cand := range agResp.Response.Candidates {
		for _, part := range cand.Content.Parts {
			if !part.Thought {
				fullText.WriteString(part.Text)
			}
		}
	}

	oaiResp := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   target.model,
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": fullText.String(),
				},
				"finish_reason": "stop",
			},
		},
	}
	writeJSON(w, http.StatusOK, oaiResp)
	return nil
}

func (h *APIHandler) streamAntigravityToSocket(
	subdomain string,
	target proxyTarget,
	messages []map[string]interface{},
	reqBody map[string]interface{},
	emitEvent func(string, interface{}),
) error {
	_ = h.ensureFreshAntigravityToken(target.conn)
	apiKey := target.conn.APIKey
	projectID := target.conn.ProjectID

	var contents []map[string]interface{}
	var systemParts []map[string]interface{}

	for _, m := range messages {
		role, _ := m["role"].(string)
		text, _ := m["content"].(string)
		if role == "system" {
			systemParts = append(systemParts, map[string]interface{}{"text": text})
		} else {
			geminiRole := "user"
			if role == "assistant" {
				geminiRole = "model"
			}
			contents = append(contents, map[string]interface{}{
				"role":  geminiRole,
				"parts": []map[string]interface{}{{"text": text}},
			})
		}
	}

	if len(contents) == 0 {
		contents = append(contents, map[string]interface{}{
			"role":  "user",
			"parts": []map[string]interface{}{{"text": "Hello"}},
		})
	}

	requestObj := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     1.0,
			"maxOutputTokens": 8192,
		},
	}
	if len(systemParts) > 0 {
		requestObj["systemInstruction"] = map[string]interface{}{
			"role":  "user",
			"parts": systemParts,
		}
	}

	reqID := fmt.Sprintf("agent/%s/%d/%s/1", uuid.New().String(), time.Now().UnixMilli(), uuid.New().String())
	envelope := map[string]interface{}{
		"project":   projectID,
		"model":     target.model,
		"userAgent": "antigravity",
		"requestId": reqID,
		"request":   requestObj,
	}

	envBytes, _ := json.Marshal(envelope)
	urlStr := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", antigravityBaseURL)

	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(envBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", antigravityUserAgent)
	req.Header.Set("X-Client-Name", "antigravity")
	req.Header.Set("X-Client-Version", "2.11.0")

	proxyAddr := ""
	if target.conn != nil {
		proxyAddr = target.conn.Socks5Proxy
	}
	client, cErr := createProxyHTTPClient(proxyAddr, 180*time.Second)
	if cErr != nil {
		return fmt.Errorf("proxy error: %w", cErr)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("rate_limited:%d", target.conn.ID)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(b), "RESOURCE_EXHAUSTED") || strings.Contains(string(b), "Quota") {
			return fmt.Errorf("rate_limited:%d", target.conn.ID)
		}
		return fmt.Errorf("antigravity upstream error (%d): %s", resp.StatusCode, string(b))
	}

	reader := bufio.NewReader(resp.Body)
	thinkFilter := newThinkingStreamFilter()

	for {
		line, rErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			var agResp struct {
				Response struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text    string `json:"text"`
								Thought bool   `json:"thought"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				} `json:"response"`
			}
			if json.Unmarshal([]byte(jsonStr), &agResp) == nil {
				for _, cand := range agResp.Response.Candidates {
					for _, part := range cand.Content.Parts {
						var chunkText string
						if part.Thought {
							chunkText = thinkFilter.ProcessReasoning(part.Text)
						} else {
							chunkText = thinkFilter.ProcessContent(part.Text)
						}
						if chunkText != "" {
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
					}
				}
			}
		}
		if rErr != nil {
			break
		}
	}

	if flush := thinkFilter.Flush(); flush != "" {
		emitEvent("chat:completion", map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"delta": map[string]interface{}{
						"content": flush,
					},
				},
			},
			"done": false,
		})
	}
	return nil
}

// ── Antigravity OAuth Handlers ──────────────────────────────────────────────

func (h *APIHandler) handleAntigravityOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	subdomain := r.URL.Query().Get("subdomain")
	if subdomain == "" {
		subdomain = extractSubdomainFromHost(r.Host)
	}

	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/callback"
	}

	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	if subdomain != "" {
		state = state + ":" + subdomain
	}

	scopes := []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/cclog",
		"https://www.googleapis.com/auth/experimentsandconfigs",
	}

	q := url.Values{}
	q.Set("client_id", getAntigravityClientID())
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")

	authURL := antigravityAuthorizeURL + "?" + q.Encode()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authUrl":     authURL,
		"state":       state,
		"redirectUri": redirectURI,
	})
}

func (h *APIHandler) handleAntigravityOAuthExchange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirectUri"`
		State       string `json:"state"`
		Subdomain   string `json:"subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	subdomain := req.Subdomain
	if subdomain == "" && strings.Contains(req.State, ":") {
		parts := strings.Split(req.State, ":")
		if len(parts) >= 2 {
			subdomain = parts[1]
		}
	}
	if subdomain == "" {
		subdomain = extractSubdomainFromHost(r.Host)
	}
	if subdomain == "" {
		writeJSONError(w, http.StatusBadRequest, "subdomain is required")
		return
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", getAntigravityClientID())
	form.Set("client_secret", getAntigravityClientSecret())
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)

	resp, err := http.Post(antigravityTokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to exchange token: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		writeJSONError(w, resp.StatusCode, "token exchange error: "+string(b))
		return
	}

	var tokenRes struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to decode token response: "+err.Error())
		return
	}

	// Fetch user email
	userReq, _ := http.NewRequest(http.MethodGet, antigravityUserInfoURL+"?alt=json", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	userReq.Header.Set("x-request-source", "local")
	userResp, err := http.DefaultClient.Do(userReq)
	var email string
	if err == nil && userResp.StatusCode == http.StatusOK {
		var userInfo struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(userResp.Body).Decode(&userInfo)
		email = userInfo.Email
		userResp.Body.Close()
	}
	if email == "" {
		email = fmt.Sprintf("antigravity-user-%d", time.Now().Unix())
	}

	// Load Code Assist
	loadBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"ideType":    9,
			"platform":   3,
			"pluginType": 2,
		},
	}
	loadBytes, _ := json.Marshal(loadBody)
	loadReq, _ := http.NewRequest(http.MethodPost, antigravityLoadCodeAssistURL, bytes.NewReader(loadBytes))
	loadReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	loadReq.Header.Set("Content-Type", "application/json")
	loadReq.Header.Set("User-Agent", antigravityUserAgent)
	loadReq.Header.Set("x-request-source", "local")

	var projectID string
	var tierID string = "legacy-tier"
	loadResp, err := http.DefaultClient.Do(loadReq)
	if err == nil {
		defer loadResp.Body.Close()
		if loadResp.StatusCode == http.StatusOK {
			var loadData struct {
				CloudAICompanionProject interface{} `json:"cloudaicompanionProject"`
				AllowedTiers            []struct {
					ID        string `json:"id"`
					IsDefault bool   `json:"isDefault"`
				} `json:"allowedTiers"`
			}
			if json.NewDecoder(loadResp.Body).Decode(&loadData) == nil {
				if s, ok := loadData.CloudAICompanionProject.(string); ok {
					projectID = s
				} else if m, ok := loadData.CloudAICompanionProject.(map[string]interface{}); ok {
					if idStr, ok := m["id"].(string); ok {
						projectID = idStr
					}
				}
				for _, t := range loadData.AllowedTiers {
					if t.IsDefault && t.ID != "" {
						tierID = strings.TrimSpace(t.ID)
						break
					}
				}
			}
		}
	}

	if projectID != "" {
		go func(token, tier string) {
			for i := 0; i < 5; i++ {
				obBody, _ := json.Marshal(map[string]interface{}{
					"tierId": tier,
					"metadata": map[string]interface{}{
						"ideType":    9,
						"platform":   3,
						"pluginType": 2,
					},
				})
				obReq, _ := http.NewRequest(http.MethodPost, antigravityOnboardUserURL, bytes.NewReader(obBody))
				obReq.Header.Set("Authorization", "Bearer "+token)
				obReq.Header.Set("Content-Type", "application/json")
				obReq.Header.Set("User-Agent", antigravityUserAgent)
				resp, e := http.DefaultClient.Do(obReq)
				if e == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						break
					}
				}
				time.Sleep(3 * time.Second)
			}
		}(tokenRes.AccessToken, tierID)
	}

	var expiresAt *time.Time
	if tokenRes.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(tokenRes.ExpiresIn) * time.Second)
		expiresAt = &exp
	}

	modelsJSONBytes, _ := json.Marshal(defaultAntigravityModels)
	id, err := h.db.UpsertAntigravityConnection(subdomain, email, tokenRes.AccessToken, tokenRes.RefreshToken, projectID, expiresAt, string(modelsJSONBytes))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save connection: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"id":        id,
		"email":     email,
		"projectId": projectID,
	})
}

func (h *APIHandler) handleAntigravityOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errStr := r.URL.Query().Get("error")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>OAuth Callback</title></head>
<body>
<div style="font-family:sans-serif;text-align:center;padding:50px;">
  <h2>Authorization Complete</h2>
  <p>You can close this window now.</p>
</div>
<script>
  const data = { code: %q, state: %q, error: %q };
  if (window.opener) {
    window.opener.postMessage({ type: "oauth_callback", data: data }, "*");
  }
  try {
    const ch = new BroadcastChannel("oauth_callback");
    ch.postMessage(data);
    ch.close();
  } catch(e) {}
  try {
    localStorage.setItem("oauth_callback", JSON.stringify({ ...data, timestamp: Date.now() }));
  } catch(e) {}
  setTimeout(() => window.close(), 1200);
</script>
</body>
</html>`, code, state, errStr)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

