package api

// Code generated from official OpenWebUI FastAPI routes and Pydantic models.
// DO NOT EDIT MANUALLY - Canonical schemas verified via TypeScript and Python AST.

// --- Chat Models ---

type OpenWebUIChatTitleIdResponse struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	UpdatedAt  int64   `json:"updated_at"`
	CreatedAt  int64   `json:"created_at"`
	LastReadAt *int64  `json:"last_read_at,omitempty"`
	Snippet    *string `json:"snippet,omitempty"`
	Active     bool    `json:"active"`
	Archived   bool    `json:"archived"`
	Pinned     bool    `json:"pinned"`
}

type OpenWebUIChatResponse struct {
	ID               string         `json:"id"`
	UserID           string         `json:"user_id"`
	Title            string         `json:"title"`
	Chat             any            `json:"chat"`
	UpdatedAt        int64          `json:"updated_at"`
	CreatedAt        int64          `json:"created_at"`
	ShareID          *string        `json:"share_id,omitempty"`
	Archived         bool           `json:"archived"`
	Pinned           bool           `json:"pinned"`
	Meta             map[string]any `json:"meta,omitempty"`
	Variables        map[string]any `json:"variables,omitempty"`
	FolderID         *string        `json:"folder_id,omitempty"`
	Tasks            []any          `json:"tasks,omitempty"`
	Summary          *string        `json:"summary,omitempty"`
	CurrentMessageID *string        `json:"current_message_id,omitempty"`
	ContextUsage     map[string]any `json:"context_usage,omitempty"`
}

type OpenWebUIChatConfigForm struct {
	ContextCompactionModel               string `json:"CONTEXT_COMPACTION_MODEL"`
	EnableContextCompaction              bool   `json:"ENABLE_CONTEXT_COMPACTION"`
	ContextCompactionTokenThreshold      int64  `json:"CONTEXT_COMPACTION_TOKEN_THRESHOLD"`
	ContextCompactionTokenCap            int64  `json:"CONTEXT_COMPACTION_TOKEN_CAP"`
	ContextCompactionRetentionPercentage int64  `json:"CONTEXT_COMPACTION_RETENTION_PERCENTAGE"`
	ContextCompactionPromptTemplate      string `json:"CONTEXT_COMPACTION_PROMPT_TEMPLATE"`
	EnableToolPermissions                bool   `json:"ENABLE_TOOL_PERMISSIONS"`
}

// --- Folder Models ---

type OpenWebUIFolderNameIdResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Meta        map[string]any `json:"meta,omitempty"`
	ParentID    *string        `json:"parent_id"`
	IsExpanded  bool           `json:"is_expanded"`
	UnreadCount int64          `json:"unread_count"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

type OpenWebUIFolderResponse struct {
	ID           string         `json:"id"`
	ParentID     *string        `json:"parent_id"`
	UserID       string         `json:"user_id"`
	Name         string         `json:"name"`
	Items        map[string]any `json:"items,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
	IsExpanded   bool           `json:"is_expanded"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
	AccessGrants []any          `json:"access_grants"`
	WriteAccess  bool           `json:"write_access"`
}

// --- Paginated Responses ---

type OpenWebUIPaginatedListResponse struct {
	Items []any `json:"items"`
	Total int64 `json:"total"`
}

// --- User Models ---

type OpenWebUISessionUserInfoResponse struct {
	Token            string          `json:"token"`
	TokenType        string          `json:"token_type"`
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Role             string          `json:"role"`
	Email            string          `json:"email"`
	ProfileImageURL  string          `json:"profile_image_url"`
	ExpiresAt        *int64          `json:"expires_at,omitempty"`
	Permissions      map[string]any  `json:"permissions,omitempty"`
	StatusEmoji      *string         `json:"status_emoji,omitempty"`
	StatusMessage    *string         `json:"status_message,omitempty"`
	StatusExpiresAt  *int64          `json:"status_expires_at,omitempty"`
	Bio              *string         `json:"bio,omitempty"`
	Gender           *string         `json:"gender,omitempty"`
	DateOfBirth      *string         `json:"date_of_birth,omitempty"`
}

type OpenWebUIUserSettings struct {
	UI map[string]any `json:"ui"`
}

type OpenWebUIUserPermissions struct {
	Workspace map[string]bool `json:"workspace"`
	Chat      map[string]bool `json:"chat"`
}

// --- Config Models ---

type OpenWebUITaskConfigForm struct {
	TaskModel                            string         `json:"TASK_MODEL"`
	TaskModelExternal                    string         `json:"TASK_MODEL_EXTERNAL"`
	TaskModelParams                      map[string]any `json:"TASK_MODEL_PARAMS,omitempty"`
	EnableTitleGeneration                bool           `json:"ENABLE_TITLE_GENERATION"`
	TitleGenerationPromptTemplate        string         `json:"TITLE_GENERATION_PROMPT_TEMPLATE"`
	ImagePromptGenerationPromptTemplate  string         `json:"IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE"`
	EnableAutocompleteGeneration         bool           `json:"ENABLE_AUTOCOMPLETE_GENERATION"`
	AutocompleteGenerationInputMaxLength int64          `json:"AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH"`
	AutocompleteGenerationPromptTemplate string         `json:"AUTOCOMPLETE_GENERATION_PROMPT_TEMPLATE"`
	TagsGenerationPromptTemplate         string         `json:"TAGS_GENERATION_PROMPT_TEMPLATE"`
	FollowUpGenerationPromptTemplate     string         `json:"FOLLOW_UP_GENERATION_PROMPT_TEMPLATE"`
	EnableFollowUpGeneration             bool           `json:"ENABLE_FOLLOW_UP_GENERATION"`
	EnableTagsGeneration                 bool           `json:"ENABLE_TAGS_GENERATION"`
	EnableSearchQueryGeneration          bool           `json:"ENABLE_SEARCH_QUERY_GENERATION"`
	EnableRetrievalQueryGeneration       bool           `json:"ENABLE_RETRIEVAL_QUERY_GENERATION"`
	QueryGenerationPromptTemplate        string         `json:"QUERY_GENERATION_PROMPT_TEMPLATE"`
	ToolsFunctionCallingPromptTemplate   string         `json:"TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE"`
	EnableVoiceModePrompt                bool           `json:"ENABLE_VOICE_MODE_PROMPT"`
	VoiceModePromptTemplate              string         `json:"VOICE_MODE_PROMPT_TEMPLATE"`
}

type OpenWebUIConnectionsConfigForm struct {
	EnableDirectConnections  bool `json:"ENABLE_DIRECT_CONNECTIONS"`
	EnableDirectIntegrations bool `json:"ENABLE_DIRECT_INTEGRATIONS"`
	EnableBaseModelsCache    bool `json:"ENABLE_BASE_MODELS_CACHE"`
}

type OpenWebUIToolServersConfigForm struct {
	ToolServerConnections []any `json:"TOOL_SERVER_CONNECTIONS"`
}

type OpenWebUIAdminConfig struct {
	ShowAdminDetails bool   `json:"SHOW_ADMIN_DETAILS"`
	AdminEmail       string `json:"ADMIN_EMAIL"`
	WebuiURL         string `json:"WEBUI_URL"`
	EnableLoginForm  bool   `json:"ENABLE_LOGIN_FORM"`
	EnableSignup     bool   `json:"ENABLE_SIGNUP"`
	EnableAPIKeys    bool   `json:"ENABLE_API_KEYS"`
	EnableFolders    bool   `json:"ENABLE_FOLDERS"`
	EnableChannels   bool   `json:"ENABLE_CHANNELS"`
}
