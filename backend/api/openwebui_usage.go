package api

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type OpenWebUIUserUsageTotals struct {
	LifetimeTokens     int64 `json:"lifetime_tokens"`
	InputTokens        int64 `json:"input_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	PeakDailyTokens    int64 `json:"peak_daily_tokens"`
	LongestChatSeconds int64 `json:"longest_chat_seconds"`
	CurrentStreak      int   `json:"current_streak"`
	LongestStreak      int   `json:"longest_streak"`
	TotalChats         int   `json:"total_chats"`
	ActiveDays         int   `json:"active_days"`
	ModelsUsed         int   `json:"models_used"`
	Messages           int   `json:"messages"`
	UserMessages       int   `json:"user_messages"`
	AssistantMessages  int   `json:"assistant_messages"`
}

type OpenWebUIUserUsageHeatmapEntry struct {
	Date     string         `json:"date"`
	Messages int            `json:"messages"`
	Chats    int            `json:"chats"`
	Tokens   int64          `json:"tokens"`
	Models   map[string]int `json:"models"`
}

type OpenWebUIUserUsageInsights struct {
	MostUsedModel               *string `json:"most_used_model"`
	AverageTokensPerChat        float64 `json:"average_tokens_per_chat"`
	AverageMessagesPerActiveDay float64 `json:"average_messages_per_active_day"`
	UserMessageShare            float64 `json:"user_message_share"`
	AssistantMessageShare       float64 `json:"assistant_message_share"`
}

type OpenWebUIUserUsageModelEntry struct {
	ModelID      string `json:"model_id"`
	Messages     int    `json:"messages"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

type OpenWebUIUserUsageToolEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type OpenWebUIUserUsagePeriod struct {
	StartDate int64 `json:"start_date"`
	EndDate   int64 `json:"end_date"`
	Days      int   `json:"days"`
}

type OpenWebUIUserUsageResponse struct {
	Totals            OpenWebUIUserUsageTotals         `json:"totals"`
	Heatmap           []OpenWebUIUserUsageHeatmapEntry `json:"heatmap"`
	WeeklyHeatmap     []OpenWebUIUserUsageHeatmapEntry `json:"weekly_heatmap"`
	CumulativeHeatmap []OpenWebUIUserUsageHeatmapEntry `json:"cumulative_heatmap"`
	Insights          OpenWebUIUserUsageInsights       `json:"insights"`
	TopModels         []OpenWebUIUserUsageModelEntry   `json:"top_models"`
	TopTools          []OpenWebUIUserUsageToolEntry    `json:"top_tools"`
	Period            OpenWebUIUserUsagePeriod         `json:"period"`
}

func extractMessagesFromChat(chatObj map[string]any, out *[]map[string]any) {
	if chatObj == nil {
		return
	}
	if c, ok := chatObj["chat"].(map[string]any); ok {
		extractMessagesFromChat(c, out)
		if len(*out) > 0 {
			return
		}
	}
	if h, ok := chatObj["history"].(map[string]any); ok {
		if msgs, ok := h["messages"].(map[string]any); ok {
			for _, m := range msgs {
				if mObj, ok := m.(map[string]any); ok {
					*out = append(*out, mObj)
				}
			}
			return
		}
		if msgs, ok := h["messages"].([]any); ok {
			for _, m := range msgs {
				if mObj, ok := m.(map[string]any); ok {
					*out = append(*out, mObj)
				}
			}
			return
		}
	}
	if msgs, ok := chatObj["messages"].(map[string]any); ok {
		for _, m := range msgs {
			if mObj, ok := m.(map[string]any); ok {
				*out = append(*out, mObj)
			}
		}
		return
	}
	if msgs, ok := chatObj["messages"].([]any); ok {
		for _, m := range msgs {
			if mObj, ok := m.(map[string]any); ok {
				*out = append(*out, mObj)
			}
		}
		return
	}
}

// handleOpenWebUIUserUsage handles GET /api/v1/users/usage and /api/v1/users/user/usage
func (h *APIHandler) handleOpenWebUIUserUsage(w http.ResponseWriter, r *http.Request, subdomain string, u *OpenWebUISessionUserInfoResponse) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	daysParam := r.URL.Query().Get("days")
	startDateParam := r.URL.Query().Get("start_date")
	endDateParam := r.URL.Query().Get("end_date")

	now := time.Now().Unix()
	periodEnd := now
	if endDateParam != "" {
		if v, err := strconv.ParseInt(endDateParam, 10, 64); err == nil && v > 0 {
			periodEnd = v
		}
	}

	days := 730
	if daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d >= 7 && d <= 732 {
			days = d
		}
	}

	var periodStart int64
	if startDateParam != "" {
		if v, err := strconv.ParseInt(startDateParam, 10, 64); err == nil && v > 0 {
			periodStart = v
		}
	}
	if periodStart == 0 {
		periodStart = periodEnd - int64((days-1)*86400)
	}
	if periodStart > periodEnd {
		periodStart = periodEnd - int64((days-1)*86400)
	}
	periodDays := int((periodEnd-periodStart)/86400) + 1
	if periodDays < 1 {
		periodDays = 1
	}

	// Fetch all user chats
	rawChats, _ := h.db.GetOpenWebUIChats(subdomain, u.ID, true, true, true)
	totalChats := len(rawChats)

	var longestChatSeconds int64 = 0
	type dailyAcc struct {
		messages int
		tokens   int64
		chatIDs  map[string]bool
		models   map[string]int
	}
	dailyMap := make(map[string]*dailyAcc)

	type modelAcc struct {
		messages     int
		inputTokens  int64
		outputTokens int64
		totalTokens  int64
	}
	modelMap := make(map[string]*modelAcc)

	var lifetimeInputTokens int64
	var lifetimeOutputTokens int64
	var lifetimeTotalTokens int64

	var periodMessages int
	var periodUserMessages int
	var periodAssistantMessages int

	for _, rc := range rawChats {
		duration := rc.UpdatedAt - rc.CreatedAt
		if duration > longestChatSeconds {
			longestChatSeconds = duration
		}

		_, chatJSON, _, _, _, _, _, err := h.db.GetOpenWebUIChatRaw(subdomain, u.ID, rc.ID)
		if err != nil || chatJSON == "" {
			continue
		}

		var chatObj map[string]any
		if err := json.Unmarshal([]byte(chatJSON), &chatObj); err != nil {
			continue
		}

		var msgList []map[string]any
		extractMessagesFromChat(chatObj, &msgList)

		for _, m := range msgList {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			model, _ := m["model"].(string)
			if model == "" {
				if mArr, ok := m["models"].([]any); ok && len(mArr) > 0 {
					model, _ = mArr[0].(string)
				}
			}

			// Extract timestamp
			var ts int64
			if tVal, ok := m["timestamp"].(float64); ok {
				ts = int64(tVal)
			} else if tVal, ok := m["created_at"].(float64); ok {
				ts = int64(tVal)
			} else if tVal, ok := m["timestamp"].(int64); ok {
				ts = tVal
			} else if tVal, ok := m["created_at"].(int64); ok {
				ts = tVal
			}
			if ts == 0 {
				ts = rc.CreatedAt
			}
			if ts > 1e11 {
				ts = ts / 1000
			}

			// Extract tokens
			var inTok, outTok, totTok int64
			if info, ok := m["info"].(map[string]any); ok {
				if usage, ok := info["usage"].(map[string]any); ok {
					if pt, ok := usage["prompt_tokens"].(float64); ok {
						inTok = int64(pt)
					}
					if ct, ok := usage["completion_tokens"].(float64); ok {
						outTok = int64(ct)
					}
					if tt, ok := usage["total_tokens"].(float64); ok {
						totTok = int64(tt)
					}
				}
			}
			if inTok == 0 && outTok == 0 && totTok == 0 {
				if usage, ok := m["usage"].(map[string]any); ok {
					if pt, ok := usage["prompt_tokens"].(float64); ok {
						inTok = int64(pt)
					}
					if ct, ok := usage["completion_tokens"].(float64); ok {
						outTok = int64(ct)
					}
					if tt, ok := usage["total_tokens"].(float64); ok {
						totTok = int64(tt)
					}
				}
			}
			if totTok == 0 {
				est := int64(len(content) / 4)
				if est < 1 {
					est = 1
				}
				if role == "assistant" {
					outTok = est
				} else {
					inTok = est
				}
				totTok = inTok + outTok
			}
			if totTok > 0 && inTok+outTok == 0 {
				inTok = totTok / 2
				outTok = totTok - inTok
			}

			lifetimeInputTokens += inTok
			lifetimeOutputTokens += outTok
			lifetimeTotalTokens += totTok

			if model != "" {
				mAcc := modelMap[model]
				if mAcc == nil {
					mAcc = &modelAcc{}
					modelMap[model] = mAcc
				}
				mAcc.messages++
				mAcc.inputTokens += inTok
				mAcc.outputTokens += outTok
				mAcc.totalTokens += totTok
			}

			// Check period
			if ts >= periodStart && ts <= periodEnd {
				dateStr := time.Unix(ts, 0).UTC().Format("2006-01-02")
				dAcc := dailyMap[dateStr]
				if dAcc == nil {
					dAcc = &dailyAcc{
						chatIDs: make(map[string]bool),
						models:  make(map[string]int),
					}
					dailyMap[dateStr] = dAcc
				}
				dAcc.messages++
				dAcc.tokens += totTok
				dAcc.chatIDs[rc.ID] = true
				if role == "assistant" && model != "" {
					dAcc.models[model]++
				}

				periodMessages++
				if role == "user" {
					periodUserMessages++
				} else if role == "assistant" {
					periodAssistantMessages++
				}
			}
		}
	}

	// Generate continuous daily heatmap from periodStart to periodEnd
	var heatmap []OpenWebUIUserUsageHeatmapEntry
	startDt := time.Unix(periodStart, 0).UTC().Truncate(24 * time.Hour)
	endDt := time.Unix(periodEnd, 0).UTC().Truncate(24 * time.Hour)

	var peakDailyTokens int64 = 0
	activeDays := 0

	for cur := startDt; !cur.After(endDt); cur = cur.AddDate(0, 0, 1) {
		dateStr := cur.Format("2006-01-02")
		dAcc := dailyMap[dateStr]
		var msgCount, chatCount int
		var tokenCount int64
		modelsCopy := make(map[string]int)

		if dAcc != nil {
			msgCount = dAcc.messages
			chatCount = len(dAcc.chatIDs)
			tokenCount = dAcc.tokens
			for k, v := range dAcc.models {
				modelsCopy[k] = v
			}
			if msgCount > 0 {
				activeDays++
			}
			if tokenCount > peakDailyTokens {
				peakDailyTokens = tokenCount
			}
		}

		heatmap = append(heatmap, OpenWebUIUserUsageHeatmapEntry{
			Date:     dateStr,
			Messages: msgCount,
			Chats:    chatCount,
			Tokens:   tokenCount,
			Models:   modelsCopy,
		})
	}

	// Calculate streaks
	longestStreak := 0
	currentRun := 0
	for _, day := range heatmap {
		if day.Messages > 0 {
			currentRun++
			if currentRun > longestStreak {
				longestStreak = currentRun
			}
		} else {
			currentRun = 0
		}
	}

	currentStreak := 0
	for i := len(heatmap) - 1; i >= 0; i-- {
		if heatmap[i].Messages <= 0 {
			break
		}
		currentStreak++
	}

	// Build weekly_heatmap
	weeksMap := make(map[string]*OpenWebUIUserUsageHeatmapEntry)
	var weekDates []string
	for _, day := range heatmap {
		t, _ := time.Parse("2006-01-02", day.Date)
		weekday := int(t.Weekday())
		daysSinceMonday := (weekday + 6) % 7
		monday := t.AddDate(0, 0, -daysSinceMonday).Format("2006-01-02")

		wEntry := weeksMap[monday]
		if wEntry == nil {
			wEntry = &OpenWebUIUserUsageHeatmapEntry{
				Date:   monday,
				Models: make(map[string]int),
			}
			weeksMap[monday] = wEntry
			weekDates = append(weekDates, monday)
		}
		wEntry.Messages += day.Messages
		wEntry.Chats += day.Chats
		wEntry.Tokens += day.Tokens
		for m, c := range day.Models {
			wEntry.Models[m] += c
		}
	}
	sort.Strings(weekDates)
	var weeklyHeatmap []OpenWebUIUserUsageHeatmapEntry
	for _, wd := range weekDates {
		weeklyHeatmap = append(weeklyHeatmap, *weeksMap[wd])
	}
	if weeklyHeatmap == nil {
		weeklyHeatmap = []OpenWebUIUserUsageHeatmapEntry{}
	}

	// Build cumulative_heatmap
	var cumulativeHeatmap []OpenWebUIUserUsageHeatmapEntry
	var cumMessages, cumChats int
	var cumTokens int64
	cumModels := make(map[string]int)
	for _, day := range heatmap {
		cumMessages += day.Messages
		cumChats += day.Chats
		cumTokens += day.Tokens
		for m, c := range day.Models {
			cumModels[m] += c
		}
		mCopy := make(map[string]int, len(cumModels))
		for k, v := range cumModels {
			mCopy[k] = v
		}
		cumulativeHeatmap = append(cumulativeHeatmap, OpenWebUIUserUsageHeatmapEntry{
			Date:     day.Date,
			Messages: cumMessages,
			Chats:    cumChats,
			Tokens:   cumTokens,
			Models:   mCopy,
		})
	}
	if cumulativeHeatmap == nil {
		cumulativeHeatmap = []OpenWebUIUserUsageHeatmapEntry{}
	}

	// Build top_models
	var topModels []OpenWebUIUserUsageModelEntry
	for mID, mAcc := range modelMap {
		topModels = append(topModels, OpenWebUIUserUsageModelEntry{
			ModelID:      mID,
			Messages:     mAcc.messages,
			InputTokens:  mAcc.inputTokens,
			OutputTokens: mAcc.outputTokens,
			TotalTokens:  mAcc.totalTokens,
		})
	}
	sort.Slice(topModels, func(i, j int) bool {
		return topModels[i].TotalTokens > topModels[j].TotalTokens
	})
	if topModels == nil {
		topModels = []OpenWebUIUserUsageModelEntry{}
	}

	var mostUsedModel *string
	if len(topModels) > 0 {
		m := topModels[0].ModelID
		mostUsedModel = &m
	}

	var avgTokensPerChat float64 = 0
	if totalChats > 0 {
		avgTokensPerChat = math.Round((float64(lifetimeTotalTokens)/float64(totalChats))*10) / 10
	}

	var avgMessagesPerActiveDay float64 = 0
	if activeDays > 0 {
		avgMessagesPerActiveDay = math.Round((float64(periodMessages)/float64(activeDays))*10) / 10
	}

	var userMessageShare float64 = 0
	var assistantMessageShare float64 = 0
	if periodMessages > 0 {
		userMessageShare = math.Round((float64(periodUserMessages)/float64(periodMessages)*100)*10) / 10
		assistantMessageShare = math.Round((float64(periodAssistantMessages)/float64(periodMessages)*100)*10) / 10
	}

	modelsUsed := len(modelMap)
	if heatmap == nil {
		heatmap = []OpenWebUIUserUsageHeatmapEntry{}
	}

	resp := OpenWebUIUserUsageResponse{
		Totals: OpenWebUIUserUsageTotals{
			LifetimeTokens:     lifetimeTotalTokens,
			InputTokens:        lifetimeInputTokens,
			OutputTokens:       lifetimeOutputTokens,
			PeakDailyTokens:    peakDailyTokens,
			LongestChatSeconds: longestChatSeconds,
			CurrentStreak:      currentStreak,
			LongestStreak:      longestStreak,
			TotalChats:         totalChats,
			ActiveDays:         activeDays,
			ModelsUsed:         modelsUsed,
			Messages:           periodMessages,
			UserMessages:       periodUserMessages,
			AssistantMessages:  periodAssistantMessages,
		},
		Heatmap:           heatmap,
		WeeklyHeatmap:     weeklyHeatmap,
		CumulativeHeatmap: cumulativeHeatmap,
		Insights: OpenWebUIUserUsageInsights{
			MostUsedModel:               mostUsedModel,
			AverageTokensPerChat:        avgTokensPerChat,
			AverageMessagesPerActiveDay: avgMessagesPerActiveDay,
			UserMessageShare:            userMessageShare,
			AssistantMessageShare:       assistantMessageShare,
		},
		TopModels: topModels,
		TopTools:  []OpenWebUIUserUsageToolEntry{},
		Period: OpenWebUIUserUsagePeriod{
			StartDate: periodStart,
			EndDate:   periodEnd,
			Days:      periodDays,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}
