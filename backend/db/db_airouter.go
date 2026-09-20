package db

import "time"

// AIRouterConfig holds the routing configuration for a user's AI Router
type AIRouterConfig struct {
	ID               int64     `json:"id"`
	Subdomain        string    `json:"subdomain"`
	InputFormat      string    `json:"input_format"`  // "openai" | "anthropic"
	OutputFormat     string    `json:"output_format"` // "openai" | "anthropic"
	RoutingRulesJSON string    `json:"routing_rules_json"`
	ProviderKeysJSON string    `json:"provider_keys_json"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// GetAIRouterConfig retrieves the AI router config for a subdomain
func (d *DB) GetAIRouterConfig(subdomain string) (*AIRouterConfig, error) {
	row := d.conn.QueryRow(`
		SELECT id, subdomain, input_format, output_format, routing_rules_json, provider_keys_json, created_at, updated_at
		FROM ai_router_configs WHERE subdomain = ?`, subdomain)

	cfg := &AIRouterConfig{}
	err := row.Scan(&cfg.ID, &cfg.Subdomain, &cfg.InputFormat, &cfg.OutputFormat,
		&cfg.RoutingRulesJSON, &cfg.ProviderKeysJSON, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// UpsertAIRouterConfig creates or updates the AI router config for a subdomain
func (d *DB) UpsertAIRouterConfig(cfg *AIRouterConfig) error {
	_, err := d.conn.Exec(`
		INSERT INTO ai_router_configs (subdomain, input_format, output_format, routing_rules_json, provider_keys_json, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(subdomain) DO UPDATE SET
			input_format = excluded.input_format,
			output_format = excluded.output_format,
			routing_rules_json = excluded.routing_rules_json,
			provider_keys_json = excluded.provider_keys_json,
			updated_at = CURRENT_TIMESTAMP`,
		cfg.Subdomain, cfg.InputFormat, cfg.OutputFormat, cfg.RoutingRulesJSON, cfg.ProviderKeysJSON)
	return err
}

// EnsureAIRouterConfig creates a default config if one doesn't exist
func (d *DB) EnsureAIRouterConfig(subdomain string) (*AIRouterConfig, error) {
	cfg, err := d.GetAIRouterConfig(subdomain)
	if err == nil {
		return cfg, nil
	}
	// Create default
	defaultCfg := &AIRouterConfig{
		Subdomain:        subdomain,
		InputFormat:      "openai",
		OutputFormat:     "openai",
		RoutingRulesJSON: `[{"condition":"always","provider":"openai","model":"gpt-4o-mini"}]`,
		ProviderKeysJSON: `{}`,
	}
	if err2 := d.UpsertAIRouterConfig(defaultCfg); err2 != nil {
		return nil, err2
	}
	return defaultCfg, nil
}
