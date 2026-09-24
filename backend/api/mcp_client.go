package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MCPClient struct {
	client *http.Client
}

func NewMCPClient() *MCPClient {
	return &MCPClient{
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
	}
}

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// DiscoverEndpoint handles SSE discovery if targetURL provides an SSE stream declaring the endpoint
func (c *MCPClient) DiscoverEndpoint(ctx context.Context, targetURL, apiKey string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return targetURL, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return targetURL, nil // Fallback to targetURL as direct POST endpoint
	}
	defer resp.Body.Close()

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(resp.Body)
		var eventType string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if eventType == "endpoint" && data != "" {
					if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
						return data, nil
					}
					// Relative URL: resolve against base of targetURL
					base := targetURL
					if idx := strings.Index(base, "://"); idx != -1 {
						slashIdx := strings.Index(base[idx+3:], "/")
						if slashIdx != -1 {
							base = base[:idx+3+slashIdx]
						}
					}
					if !strings.HasPrefix(data, "/") {
						data = "/" + data
					}
					return base + data, nil
				}
			} else if line == "" {
				eventType = ""
			}
		}
	}

	return targetURL, nil
}

// CallRPC sends a JSON-RPC request to the MCP endpoint
func (c *MCPClient) CallRPC(ctx context.Context, postURL, apiKey, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(respBytes)
	// Handle SSE formatted responses containing data: {...}
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		lines := strings.Split(string(trimmed), "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "data:") {
				trimmed = []byte(strings.TrimSpace(strings.TrimPrefix(l, "data:")))
				break
			}
		}
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(trimmed, &rpcResp); err != nil {
		return nil, fmt.Errorf("invalid json-rpc response: %s", string(respBytes))
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// ListTools connects (initialize + tools/list) and returns the tool specs
func (c *MCPClient) ListTools(ctx context.Context, targetURL, apiKey string) ([]map[string]any, error) {
	postURL, _ := c.DiscoverEndpoint(ctx, targetURL, apiKey)

	// 1. Initialize
	_, _ = c.CallRPC(ctx, postURL, apiKey, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "zcdns-mcp",
			"version": "1.0",
		},
	})

	// Optional notifications/initialized
	_, _ = c.CallRPC(ctx, postURL, apiKey, "notifications/initialized", map[string]any{})

	// 2. tools/list
	rawTools, err := c.CallRPC(ctx, postURL, apiKey, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	var listResult struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(rawTools, &listResult); err != nil {
		return nil, err
	}
	return listResult.Tools, nil
}

// ExecuteTool calls a tool on the MCP server
func (c *MCPClient) ExecuteTool(ctx context.Context, targetURL, apiKey, toolName string, arguments any) (map[string]any, error) {
	postURL, _ := c.DiscoverEndpoint(ctx, targetURL, apiKey)
	rawResult, err := c.CallRPC(ctx, postURL, apiKey, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}
	var res map[string]any
	_ = json.Unmarshal(rawResult, &res)
	return res, nil
}
