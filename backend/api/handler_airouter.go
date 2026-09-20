package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"zcdns-backend/db"
)

// ── Config endpoints ──────────────────────────────────────────────────────────

func (h *APIHandler) handleGetAIRouterConfig(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	subdomain := h.getSubdomain(r)
	if subdomain == "" {
		writeJSONError(w, http.StatusUnauthorized, "subdomain required")
		return
	}
	cfg, err := h.db.EnsureAIRouterConfig(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *APIHandler) handleSaveAIRouterConfig(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	subdomain := h.getSubdomain(r)
	if subdomain == "" {
		writeJSONError(w, http.StatusUnauthorized, "subdomain required")
		return
	}
	var body struct {
		InputFormat      string `json:"input_format"`
		OutputFormat     string `json:"output_format"`
		RoutingRulesJSON string `json:"routing_rules_json"`
		ProviderKeysJSON string `json:"provider_keys_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg := &db.AIRouterConfig{
		Subdomain:        subdomain,
		InputFormat:      body.InputFormat,
		OutputFormat:     body.OutputFormat,
		RoutingRulesJSON: body.RoutingRulesJSON,
		ProviderKeysJSON: body.ProviderKeysJSON,
	}
	if cfg.InputFormat == "" {
		cfg.InputFormat = "openai"
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "openai"
	}
	if cfg.RoutingRulesJSON == "" {
		cfg.RoutingRulesJSON = "[]"
	}
	if cfg.ProviderKeysJSON == "" {
		cfg.ProviderKeysJSON = "{}"
	}
	if err := h.db.UpsertAIRouterConfig(cfg); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Proxy endpoints ───────────────────────────────────────────────────────────

// RoutingRule represents a single routing rule
type RoutingRule struct {
	Condition string `json:"condition"` // "always" | "if_model_contains:{text}" | "fallback"
	Provider  string `json:"provider"`  // "openai" | "anthropic" | "custom"
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"` // for custom provider
}

// ProviderKeys holds API keys per provider
type ProviderKeys struct {
	OpenAI    string `json:"openai"`
	Anthropic string `json:"anthropic"`
	Custom    string `json:"custom"`
	CustomURL string `json:"custom_url"`
}

func extractSubdomainFromHost(host string) string {
	// host can be "myname.router.zcdns.id" or "myname.router.zcdns.id:port"
	host = strings.Split(host, ":")[0]
	parts := strings.Split(host, ".")
	if len(parts) >= 4 && parts[1] == "router" {
		return parts[0]
	}
	return ""
}

func pickProvider(rules []RoutingRule, requestedModel string) *RoutingRule {
	var fallback *RoutingRule
	for i := range rules {
		r := &rules[i]
		switch {
		case r.Condition == "always":
			return r
		case strings.HasPrefix(r.Condition, "if_model_contains:"):
			keyword := strings.TrimPrefix(r.Condition, "if_model_contains:")
			if strings.Contains(strings.ToLower(requestedModel), strings.ToLower(keyword)) {
				return r
			}
		case r.Condition == "fallback":
			fallback = r
		}
	}
	return fallback
}

// handleAIRouterProxy is the core proxy handler for both OpenAI and Anthropic formats
func (h *APIHandler) handleAIRouterProxy(w http.ResponseWriter, r *http.Request, isAnthropic bool) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract subdomain from Host header (*.router.zcdns.id)
	subdomain := extractSubdomainFromHost(r.Host)
	if subdomain == "" {
		// Fallback: try path /api/airouter/{subdomain}/...
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

	cfg, err := h.db.EnsureAIRouterConfig(subdomain)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "router not configured")
		return
	}

	// Parse routing rules
	var rules []RoutingRule
	if err := json.Unmarshal([]byte(cfg.RoutingRulesJSON), &rules); err != nil || len(rules) == 0 {
		rules = []RoutingRule{{Condition: "always", Provider: "openai", Model: "gpt-4o-mini"}}
	}

	// Parse provider keys
	var keys ProviderKeys
	_ = json.Unmarshal([]byte(cfg.ProviderKeysJSON), &keys)

	// Read and parse request body to get requested model
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var reqBody map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &reqBody)
	requestedModel, _ := reqBody["model"].(string)

	// Pick provider based on rules
	rule := pickProvider(rules, requestedModel)
	if rule == nil {
		rule = &RoutingRule{Provider: "openai", Model: "gpt-4o-mini"}
	}

	// Override model if rule specifies one
	if rule.Model != "" && requestedModel != rule.Model {
		reqBody["model"] = rule.Model
		bodyBytes, _ = json.Marshal(reqBody)
	}

	// Build upstream URL and set API key
	var upstreamURL, apiKey string
	switch rule.Provider {
	case "anthropic":
		apiKey = keys.Anthropic
		if isAnthropic {
			upstreamURL = "https://api.anthropic.com/v1/messages"
		} else {
			upstreamURL = "https://api.anthropic.com/v1/messages"
		}
	case "custom":
		apiKey = keys.Custom
		baseURL := rule.BaseURL
		if baseURL == "" {
			baseURL = keys.CustomURL
		}
		if isAnthropic {
			upstreamURL = strings.TrimRight(baseURL, "/") + "/v1/messages"
		} else {
			upstreamURL = strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
		}
	default: // openai
		apiKey = keys.OpenAI
		if isAnthropic {
			// Convert to OpenAI format and forward
			upstreamURL = "https://api.openai.com/v1/chat/completions"
		} else {
			upstreamURL = "https://api.openai.com/v1/chat/completions"
		}
	}

	if apiKey == "" {
		writeJSONError(w, http.StatusUnauthorized, fmt.Sprintf("no API key configured for provider '%s'. Go to Dashboard → AI Router → Provider Keys", rule.Provider))
		return
	}

	// Forward request
	req, err := http.NewRequest(r.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create upstream request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if rule.Provider == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Check if streaming
	isStream, _ := reqBody["stream"].(bool)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	h.setCorsHeaders(w)
	w.WriteHeader(resp.StatusCode)

	if isStream {
		// Stream response directly
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}
}

func (h *APIHandler) handleAIRouterOpenAI(w http.ResponseWriter, r *http.Request) {
	h.handleAIRouterProxy(w, r, false)
}

func (h *APIHandler) handleAIRouterAnthropic(w http.ResponseWriter, r *http.Request) {
	h.handleAIRouterProxy(w, r, true)
}
