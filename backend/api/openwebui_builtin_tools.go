package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zcdns-backend/db"
)

// BuiltinToolSpec defines an OpenAI-compatible function tool definition
type BuiltinToolSpec struct {
	Type     string         `json:"type"`
	Function map[string]any `json:"function"`
}

// GetBuiltinToolSpecs returns all active built-in tools supported by ZCDNS OpenWebUI
// NOTE: create_tasks, update_task, create_automation, and search_calendar_events
// are excluded because they are not yet supported.
func GetBuiltinToolSpecs() []BuiltinToolSpec {
	return []BuiltinToolSpec{
		// ── B. Waktu & Utilitas ──────────────────────────────────────────
		{
			Type: "function",
			Function: map[string]any{
				"name":        "get_current_timestamp",
				"description": "Get the current date, time, Unix timestamp, and timezone information.",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "calculate_timestamp",
				"description": "Calculate a past or future Unix timestamp and date relative to current time or given reference date.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"days_ago": map[string]any{
							"type":        "integer",
							"description": "Number of days to subtract from current time (default: 0)",
						},
						"weeks_ago": map[string]any{
							"type":        "integer",
							"description": "Number of weeks to subtract from current time (default: 0)",
						},
						"months_ago": map[string]any{
							"type":        "integer",
							"description": "Number of months to subtract from current time (default: 0)",
						},
						"years_ago": map[string]any{
							"type":        "integer",
							"description": "Number of years to subtract from current time (default: 0)",
						},
						"days_ahead": map[string]any{
							"type":        "integer",
							"description": "Number of days to add into the future (default: 0)",
						},
						"relative_to": map[string]any{
							"type":        "string",
							"description": "Optional ISO 8601 date string to calculate from (defaults to now)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "ask_user",
				"description": "Ask the user clarifying questions before continuing when information is ambiguous or decisions are needed.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"questions": map[string]any{
							"type":        "array",
							"description": "1 to 3 clarifying questions with id, question text, and selectable options.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":       map[string]any{"type": "string"},
									"question": map[string]any{"type": "string"},
									"options": map[string]any{
										"type": "array",
										"items": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"label":       map[string]any{"type": "string"},
												"description": map[string]any{"type": "string"},
											},
											"required": []string{"label"},
										},
									},
								},
								"required": []string{"id", "question", "options"},
							},
						},
						"allow_other": map[string]any{
							"type":        "boolean",
							"description": "Whether users may enter a free-form answer instead of choosing options",
						},
					},
					"required": []string{"questions"},
				},
			},
		},

		// ── C. Riwayat Percakapan & Memori ──────────────────────────────
		{
			Type: "function",
			Function: map[string]any{
				"name":        "search_chats",
				"description": "Search user's past chat conversations by keywords in title and messages.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search keyword or phrase",
						},
						"count": map[string]any{
							"type":        "integer",
							"description": "Maximum number of chat results to return (default: 5)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "view_chat",
				"description": "Get the full conversation history of a specific chat session by its ID.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"chat_id": map[string]any{
							"type":        "string",
							"description": "The ID of the chat session to view",
						},
					},
					"required": []string{"chat_id"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "list_memories",
				"description": "List all stored user memories, saved preferences, and persistent facts.",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "search_memories",
				"description": "Search saved long-term memories by keyword query.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Keyword to search memories",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "add_memory",
				"description": "Save a new persistent fact or preference into user's long-term memory across sessions.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{
							"type":        "string",
							"description": "The memory or fact content to remember",
						},
					},
					"required": []string{"content"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "update_memory",
				"description": "Update an existing saved memory by its ID.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "The ID of the memory to update",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The updated content of the memory",
						},
					},
					"required": []string{"id", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "delete_memory",
				"description": "Delete a saved memory by its ID.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "The ID of the memory to delete",
						},
					},
					"required": []string{"id"},
				},
			},
		},

		// ── D. File & Dokumen Workspace ─────────────────────────────────
		{
			Type: "function",
			Function: map[string]any{
				"name":        "list_chat_files",
				"description": "List all uploaded files attached to this workspace or user session.",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "view_file",
				"description": "View text content and metadata of an uploaded file by its file ID.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_id": map[string]any{
							"type":        "string",
							"description": "The ID of the file to read",
						},
					},
					"required": []string{"file_id"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "query_chat_files",
				"description": "Search and query text across uploaded workspace files.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search query or keyword",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "search_knowledge_bases",
				"description": "Search user's knowledge bases and collections by keyword.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Keyword to search knowledge bases",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "query_knowledge_files",
				"description": "Search documents and files inside knowledge bases.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search query",
						},
						"collection_id": map[string]any{
							"type":        "string",
							"description": "Optional collection or knowledge ID to limit search",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "view_knowledge_file",
				"description": "View details and content of a file within a knowledge base.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_id": map[string]any{
							"type":        "string",
							"description": "File ID",
						},
					},
					"required": []string{"file_id"},
				},
			},
		},

		// ── E. Image Generation & Edits ─────────────────────────────────
		{
			Type: "function",
			Function: map[string]any{
				"name":        "generate_image",
				"description": "Generate an image from a detailed descriptive prompt using DALL-E, ComfyUI, or Automatic1111.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"prompt": map[string]any{
							"type":        "string",
							"description": "Detailed text description of the image to generate",
						},
						"size": map[string]any{
							"type":        "string",
							"description": "Image dimensions, e.g. 1024x1024, 512x512, 1792x1024",
						},
						"model": map[string]any{
							"type":        "string",
							"description": "Optional image generation model (e.g. dall-e-3)",
						},
					},
					"required": []string{"prompt"},
				},
			},
		},
		{
			Type: "function",
			Function: map[string]any{
				"name":        "edit_image",
				"description": "Edit or transform an existing image based on a prompt and base image URL or file ID.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"prompt": map[string]any{
							"type":        "string",
							"description": "Edit instructions describing changes to make",
						},
						"image": map[string]any{
							"type":        "string",
							"description": "Base image URL or uploaded File ID",
						},
						"size": map[string]any{
							"type":        "string",
							"description": "Image dimensions, e.g. 1024x1024",
						},
					},
					"required": []string{"prompt", "image"},
				},
			},
		},
	}
}

// ConvertBuiltinToolsToAnthropic transforms OpenAI-format tool specs to Anthropic format
func ConvertBuiltinToolsToAnthropic(specs []BuiltinToolSpec) []map[string]any {
	var antTools []map[string]any
	for _, spec := range specs {
		fn := spec.Function
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params, _ := fn["parameters"].(map[string]any)
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		antTools = append(antTools, map[string]any{
			"name":         name,
			"description":  desc,
			"input_schema": params,
		})
	}
	return antTools
}

// ExecuteBuiltinTool executes one of the built-in tools in Go
func (h *APIHandler) ExecuteBuiltinTool(ctx context.Context, toolName string, args map[string]any, subdomain, userID, currentChatID string) (any, error) {
	now := time.Now()

	switch toolName {
	// ── B. Waktu & Utilitas ──────────────────────────────────────────
	case "get_current_timestamp":
		zoneName, zoneOffset := now.Zone()
		return map[string]any{
			"current_timestamp": now.Unix(),
			"current_iso":       now.UTC().Format(time.RFC3339),
			"user_local_iso":    now.Format("2006-01-02T15:04:05-07:00"),
			"date_human":        now.Format("Monday, 02 January 2006"),
			"time_human":        now.Format("15:04:05 MST"),
			"timezone":          zoneName,
			"utc_offset_sec":    zoneOffset,
		}, nil

	case "calculate_timestamp":
		daysAgo, _ := args["days_ago"].(float64)
		weeksAgo, _ := args["weeks_ago"].(float64)
		monthsAgo, _ := args["months_ago"].(float64)
		yearsAgo, _ := args["years_ago"].(float64)
		daysAhead, _ := args["days_ahead"].(float64)

		baseTime := now
		if relStr, ok := args["relative_to"].(string); ok && relStr != "" {
			if t, err := time.Parse(time.RFC3339, relStr); err == nil {
				baseTime = t
			} else if t, err := time.Parse("2006-01-02", relStr); err == nil {
				baseTime = t
			}
		}

		totalDaysSubtract := int(daysAgo) + (int(weeksAgo) * 7) + (int(monthsAgo) * 30) + (int(yearsAgo) * 365)
		calcTime := baseTime.AddDate(0, 0, -totalDaysSubtract)
		if daysAhead > 0 {
			calcTime = calcTime.AddDate(0, 0, int(daysAhead))
		}

		return map[string]any{
			"base_timestamp":       baseTime.Unix(),
			"base_iso":             baseTime.UTC().Format(time.RFC3339),
			"calculated_timestamp": calcTime.Unix(),
			"calculated_iso":       calcTime.UTC().Format(time.RFC3339),
			"date_human":           calcTime.Format("Monday, 02 January 2006"),
		}, nil

	case "ask_user":
		questions, _ := args["questions"].([]any)
		allowOther, _ := args["allow_other"].(bool)
		return map[string]any{
			"status":      "prompt_required",
			"questions":   questions,
			"allow_other": allowOther,
			"message":     "Clarification prompt presented to user.",
		}, nil

	// ── C. Riwayat Percakapan & Memori ──────────────────────────────
	case "search_chats":
		query, _ := args["query"].(string)
		count := 5
		if cFloat, ok := args["count"].(float64); ok && cFloat > 0 {
			count = int(cFloat)
		}
		if count > 20 {
			count = 20
		}
		chats, err := h.db.GetOpenWebUIChats(subdomain, userID, false, true, true)
		if err != nil {
			return nil, err
		}
		var results []map[string]any
		qLower := strings.ToLower(query)
		for _, c := range chats {
			if c.ID == currentChatID {
				continue
			}
			tMatch := strings.Contains(strings.ToLower(c.Title), qLower)
			if tMatch || qLower == "" {
				results = append(results, map[string]any{
					"id":         c.ID,
					"title":      c.Title,
					"updated_at": c.UpdatedAt,
				})
				if len(results) >= count {
					break
				}
			}
		}
		if results == nil {
			results = []map[string]any{}
		}
		return results, nil

	case "view_chat":
		targetChatID, _ := args["chat_id"].(string)
		if targetChatID == "" {
			return map[string]any{"error": "chat_id is required"}, nil
		}
		title, rawJSON, _, _, _, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, userID, targetChatID)
		if err != nil {
			return map[string]any{"error": "chat not found or access denied: " + err.Error()}, nil
		}
		var chatObj map[string]any
		_ = json.Unmarshal([]byte(rawJSON), &chatObj)
		messages := []map[string]any{}
		if cInner, ok := chatObj["chat"].(map[string]any); ok {
			if history, ok := cInner["history"].(map[string]any); ok {
				if msgMap, ok := history["messages"].(map[string]any); ok {
					for _, m := range msgMap {
						if mObj, ok := m.(map[string]any); ok {
							role, _ := mObj["role"].(string)
							content, _ := mObj["content"].(string)
							messages = append(messages, map[string]any{
								"role":    role,
								"content": content,
							})
						}
					}
				}
			}
		}
		return map[string]any{
			"chat_id":  targetChatID,
			"title":    title,
			"messages": messages,
		}, nil

	case "list_memories":
		memories, err := h.db.GetOpenWebUIMemories(subdomain, userID)
		if err != nil {
			return nil, err
		}
		return memories, nil

	case "search_memories":
		query, _ := args["query"].(string)
		memories, err := h.db.SearchOpenWebUIMemories(subdomain, userID, query)
		if err != nil {
			return nil, err
		}
		return memories, nil

	case "add_memory":
		content, _ := args["content"].(string)
		if strings.TrimSpace(content) == "" {
			return map[string]any{"error": "content cannot be empty"}, nil
		}
		mID := fmt.Sprintf("mem-%d", time.Now().UnixNano())
		m := db.OpenWebUIMemoryDB{
			ID:        mID,
			Subdomain: subdomain,
			UserID:    userID,
			Content:   content,
			Type:      "context",
		}
		if err := h.db.InsertOpenWebUIMemory(m); err != nil {
			return nil, err
		}
		return map[string]any{"status": "success", "id": mID, "content": content}, nil

	case "update_memory":
		mID, _ := args["id"].(string)
		content, _ := args["content"].(string)
		if mID == "" || strings.TrimSpace(content) == "" {
			return map[string]any{"error": "id and content are required"}, nil
		}
		if err := h.db.UpdateOpenWebUIMemory(subdomain, userID, mID, content); err != nil {
			return nil, err
		}
		return map[string]any{"status": "success", "id": mID, "content": content}, nil

	case "delete_memory":
		mID, _ := args["id"].(string)
		if mID == "" {
			return map[string]any{"error": "id is required"}, nil
		}
		if err := h.db.DeleteOpenWebUIMemory(subdomain, userID, mID); err != nil {
			return nil, err
		}
		return map[string]any{"status": "success", "id": mID}, nil

	// ── D. File & Dokumen Workspace ─────────────────────────────────
	case "list_chat_files":
		files, _, err := h.db.GetOpenWebUIFiles(subdomain, userID, 0, 50)
		if err != nil {
			return nil, err
		}
		var results []map[string]any
		for _, f := range files {
			results = append(results, map[string]any{
				"id":           f.ID,
				"filename":     f.Filename,
				"content_type": f.ContentType,
				"size":         f.Size,
				"created_at":   f.CreatedAt,
			})
		}
		if results == nil {
			results = []map[string]any{}
		}
		return results, nil

	case "view_file":
		fileID, _ := args["file_id"].(string)
		if fileID == "" {
			return map[string]any{"error": "file_id is required"}, nil
		}
		f, err := h.db.GetOpenWebUIFileByID(subdomain, fileID)
		if err != nil || f == nil {
			return map[string]any{"error": "file not found"}, nil
		}

		// Read file contents if stored on disk
		contentSnippet := ""
		filePath := f.Path
		if filePath == "" {
			filePath = filepath.Join(h.getUploadsDir(subdomain), f.ID+"_"+f.Filename)
		}
		if data, err := os.ReadFile(filePath); err == nil {
			if len(data) > 8000 {
				contentSnippet = string(data[:8000]) + "\n...[truncated]"
			} else {
				contentSnippet = string(data)
			}
		} else {
			contentSnippet = f.DataJSON
		}

		return map[string]any{
			"id":           f.ID,
			"filename":     f.Filename,
			"content_type": f.ContentType,
			"size":         f.Size,
			"content":      contentSnippet,
		}, nil

	case "query_chat_files":
		query, _ := args["query"].(string)
		files, err := h.db.SearchOpenWebUIFiles(subdomain, userID, query, 0, 20)
		if err != nil {
			return nil, err
		}
		var results []map[string]any
		for _, f := range files {
			results = append(results, map[string]any{
				"id":           f.ID,
				"filename":     f.Filename,
				"content_type": f.ContentType,
				"size":         f.Size,
			})
		}
		if results == nil {
			results = []map[string]any{}
		}
		return results, nil

	case "search_knowledge_bases":
		query, _ := args["query"].(string)
		kbs, err := h.db.GetOpenWebUIKnowledgeBases(subdomain, userID)
		if err != nil {
			return nil, err
		}
		var results []map[string]any
		qLower := strings.ToLower(query)
		for _, k := range kbs {
			if strings.Contains(strings.ToLower(k.Name), qLower) || strings.Contains(strings.ToLower(k.Description), qLower) || qLower == "" {
				results = append(results, map[string]any{
					"id":          k.ID,
					"name":        k.Name,
					"description": k.Description,
					"created_at":  k.CreatedAt,
				})
			}
		}
		if results == nil {
			results = []map[string]any{}
		}
		return results, nil

	case "query_knowledge_files", "view_knowledge_file":
		fID, _ := args["file_id"].(string)
		query, _ := args["query"].(string)
		if fID != "" {
			f, err := h.db.GetOpenWebUIFileByID(subdomain, fID)
			if err != nil || f == nil {
				return map[string]any{"error": "knowledge file not found"}, nil
			}
			return map[string]any{
				"id":           f.ID,
				"filename":     f.Filename,
				"content_type": f.ContentType,
				"size":         f.Size,
			}, nil
		}
		files, err := h.db.SearchOpenWebUIFiles(subdomain, userID, query, 0, 10)
		if err != nil {
			return nil, err
		}
		return files, nil

	// ── E. Image Generation & Edits ─────────────────────────────────
	case "generate_image", "edit_image":
		prompt, _ := args["prompt"].(string)
		size, _ := args["size"].(string)
		if size == "" {
			size = "1024x1024"
		}
		model, _ := args["model"].(string)
		if model == "" {
			model = "dall-e-3"
		}

		imgResult, err := h.dispatchImageGeneration(ctx, subdomain, toolName, prompt, size, model, args)
		if err != nil {
			return map[string]any{"error": "image generation failed: " + err.Error()}, nil
		}
		return imgResult, nil
	}

	return nil, fmt.Errorf("unknown builtin tool: %s", toolName)
}

// dispatchImageGeneration calls OpenAI/custom image generation backend
func (h *APIHandler) dispatchImageGeneration(ctx context.Context, subdomain, action, prompt, size, model string, extraArgs map[string]any) (any, error) {
	conns, err := h.db.GetAIRouterConnections(subdomain)
	if err != nil || len(conns) == 0 {
		return nil, fmt.Errorf("no AI router connections configured for subdomain %s", subdomain)
	}

	var apiKey, baseURL, socks5Proxy string
	for _, c := range conns {
		if c.Provider == "openai" && c.APIKey != "" {
			apiKey = c.APIKey
			baseURL = c.BaseURL
			socks5Proxy = c.Socks5Proxy
			break
		}
		if (c.Provider == "custom" || c.APIType == "openai") && c.APIKey != "" && apiKey == "" {
			apiKey = c.APIKey
			baseURL = c.BaseURL
			socks5Proxy = c.Socks5Proxy
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no OpenAI or custom API key configured in AI Router settings for image generation")
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	endpoint := baseURL + "/images/generations"
	if action == "edit_image" {
		endpoint = baseURL + "/images/edits"
	}

	payload := map[string]any{
		"prompt": prompt,
		"model":  model,
		"n":      1,
		"size":   size,
	}
	for k, v := range extraArgs {
		if k != "prompt" && k != "model" && k != "size" {
			payload[k] = v
		}
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client, cErr := createProxyHTTPClient(socks5Proxy, 90*time.Second)
	if cErr != nil {
		return nil, fmt.Errorf("proxy error: %w", cErr)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return string(respBytes), nil
	}
	return result, nil
}
