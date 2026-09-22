package db

import "time"

// ── AI Router Config (per subdomain) ─────────────────────────────────────────

type AIRouterConfig struct {
	ID               int64     `json:"id"`
	Subdomain        string    `json:"subdomain"`
	InputFormat      string    `json:"input_format"`  // "openai" | "anthropic" | "auto"
	OutputFormat     string    `json:"output_format"` // "openai" | "anthropic"
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ── Provider Connections (multiple keys per provider) ─────────────────────────

type AIRouterConnection struct {
	ID        int64     `json:"id"`
	Subdomain string    `json:"subdomain"`
	Provider  string    `json:"provider"`   // "openai" | "anthropic" | "groq" | "together" | "custom"
	Name      string    `json:"name"`       // friendly label e.g. "Personal OpenAI"
	APIKey    string    `json:"api_key"`
	BaseURL   string    `json:"base_url"`   // only for custom providers
	Status    string    `json:"status"`     // "active" | "rate_limited" | "error"
	RateLimitedUntil *time.Time `json:"rate_limited_until,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Combos (named group of models with strategy) ──────────────────────────────

type AIRouterCombo struct {
	ID        int64     `json:"id"`
	Subdomain string    `json:"subdomain"`
	Name      string    `json:"name"`       // e.g. "best-coding"
	Strategy  string    `json:"strategy"`   // "fallback" | "round_robin" | "round_robin_sticky"
	ModelsJSON string   `json:"models_json"` // JSON array of "provider/model" strings
	CreatedAt time.Time `json:"created_at"`
}

// ── Model Aliases ─────────────────────────────────────────────────────────────

type AIRouterAlias struct {
	ID         int64     `json:"id"`
	Subdomain  string    `json:"subdomain"`
	AliasName  string    `json:"alias_name"`   // what user sends, e.g. "claude"
	TargetModel string   `json:"target_model"` // "anthropic/claude-3-5-sonnet-20241022" or combo name
	CreatedAt  time.Time `json:"created_at"`
}

// ── DB Methods ────────────────────────────────────────────────────────────────

// Config
func (d *DB) GetAIRouterConfig(subdomain string) (*AIRouterConfig, error) {
	row := d.conn.QueryRow(`SELECT id, subdomain, input_format, output_format, created_at, updated_at FROM ai_router_configs WHERE subdomain = ?`, subdomain)
	cfg := &AIRouterConfig{}
	err := row.Scan(&cfg.ID, &cfg.Subdomain, &cfg.InputFormat, &cfg.OutputFormat, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (d *DB) EnsureAIRouterConfig(subdomain string) (*AIRouterConfig, error) {
	cfg, err := d.GetAIRouterConfig(subdomain)
	if err == nil {
		return cfg, nil
	}
	_, err = d.conn.Exec(`INSERT OR IGNORE INTO ai_router_configs (subdomain, input_format, output_format) VALUES (?, 'auto', 'openai')`, subdomain)
	if err != nil {
		return nil, err
	}
	return d.GetAIRouterConfig(subdomain)
}

func (d *DB) UpdateAIRouterConfig(subdomain, inputFormat, outputFormat string) error {
	_, err := d.conn.Exec(`UPDATE ai_router_configs SET input_format=?, output_format=?, updated_at=CURRENT_TIMESTAMP WHERE subdomain=?`,
		inputFormat, outputFormat, subdomain)
	return err
}

// Connections
func (d *DB) GetAIRouterConnections(subdomain string) ([]AIRouterConnection, error) {
	rows, err := d.conn.Query(`SELECT id, subdomain, provider, name, api_key, base_url, status, rate_limited_until, created_at FROM ai_router_connections WHERE subdomain = ? ORDER BY provider, name`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conns []AIRouterConnection
	for rows.Next() {
		var c AIRouterConnection
		if err := rows.Scan(&c.ID, &c.Subdomain, &c.Provider, &c.Name, &c.APIKey, &c.BaseURL, &c.Status, &c.RateLimitedUntil, &c.CreatedAt); err != nil {
			return nil, err
		}
		// Mask API key
		if len(c.APIKey) > 8 {
			c.APIKey = c.APIKey[:4] + "..." + c.APIKey[len(c.APIKey)-4:]
		}
		conns = append(conns, c)
	}
	return conns, nil
}

func (d *DB) AddAIRouterConnection(conn *AIRouterConnection) (int64, error) {
	res, err := d.conn.Exec(`INSERT INTO ai_router_connections (subdomain, provider, name, api_key, base_url, status) VALUES (?, ?, ?, ?, ?, 'active')`,
		conn.Subdomain, conn.Provider, conn.Name, conn.APIKey, conn.BaseURL)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) DeleteAIRouterConnection(subdomain string, id int64) error {
	_, err := d.conn.Exec(`DELETE FROM ai_router_connections WHERE id=? AND subdomain=?`, id, subdomain)
	return err
}

func (d *DB) GetActiveAIRouterConnection(subdomain, provider string, excludeIDs []int64) (*AIRouterConnection, error) {
	// Build exclusion list
	excludeClause := ""
	args := []interface{}{subdomain, provider}
	for _, id := range excludeIDs {
		excludeClause += " AND id != ?"
		args = append(args, id)
	}
	row := d.conn.QueryRow(`SELECT id, subdomain, provider, name, api_key, base_url, status, rate_limited_until, created_at
		FROM ai_router_connections
		WHERE subdomain=? AND provider=? AND status='active'`+excludeClause+`
		ORDER BY RANDOM() LIMIT 1`, args...)
	var c AIRouterConnection
	if err := row.Scan(&c.ID, &c.Subdomain, &c.Provider, &c.Name, &c.APIKey, &c.BaseURL, &c.Status, &c.RateLimitedUntil, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) MarkAIRouterConnectionRateLimited(id int64) error {
	_, err := d.conn.Exec(`UPDATE ai_router_connections SET status='rate_limited', rate_limited_until=datetime('now','+60 seconds') WHERE id=?`, id)
	return err
}

func (d *DB) ReactivateExpiredAIRouterConnections() {
	d.conn.Exec(`UPDATE ai_router_connections SET status='active', rate_limited_until=NULL WHERE status='rate_limited' AND rate_limited_until < CURRENT_TIMESTAMP`)
}

// Combos
func (d *DB) GetAIRouterCombos(subdomain string) ([]AIRouterCombo, error) {
	rows, err := d.conn.Query(`SELECT id, subdomain, name, strategy, models_json, created_at FROM ai_router_combos WHERE subdomain=? ORDER BY name`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var combos []AIRouterCombo
	for rows.Next() {
		var c AIRouterCombo
		if err := rows.Scan(&c.ID, &c.Subdomain, &c.Name, &c.Strategy, &c.ModelsJSON, &c.CreatedAt); err != nil {
			return nil, err
		}
		combos = append(combos, c)
	}
	return combos, nil
}

func (d *DB) UpsertAIRouterCombo(combo *AIRouterCombo) error {
	_, err := d.conn.Exec(`INSERT INTO ai_router_combos (subdomain, name, strategy, models_json)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(subdomain, name) DO UPDATE SET strategy=excluded.strategy, models_json=excluded.models_json`,
		combo.Subdomain, combo.Name, combo.Strategy, combo.ModelsJSON)
	return err
}

func (d *DB) DeleteAIRouterCombo(subdomain string, id int64) error {
	_, err := d.conn.Exec(`DELETE FROM ai_router_combos WHERE id=? AND subdomain=?`, id, subdomain)
	return err
}

func (d *DB) GetAIRouterComboByName(subdomain, name string) (*AIRouterCombo, error) {
	row := d.conn.QueryRow(`SELECT id, subdomain, name, strategy, models_json, created_at FROM ai_router_combos WHERE subdomain=? AND name=?`, subdomain, name)
	var c AIRouterCombo
	if err := row.Scan(&c.ID, &c.Subdomain, &c.Name, &c.Strategy, &c.ModelsJSON, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// Aliases
func (d *DB) GetAIRouterAliases(subdomain string) ([]AIRouterAlias, error) {
	rows, err := d.conn.Query(`SELECT id, subdomain, alias_name, target_model, created_at FROM ai_router_aliases WHERE subdomain=? ORDER BY alias_name`, subdomain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aliases []AIRouterAlias
	for rows.Next() {
		var a AIRouterAlias
		if err := rows.Scan(&a.ID, &a.Subdomain, &a.AliasName, &a.TargetModel, &a.CreatedAt); err != nil {
			return nil, err
		}
		aliases = append(aliases, a)
	}
	return aliases, nil
}

func (d *DB) UpsertAIRouterAlias(alias *AIRouterAlias) error {
	_, err := d.conn.Exec(`INSERT INTO ai_router_aliases (subdomain, alias_name, target_model)
		VALUES (?, ?, ?)
		ON CONFLICT(subdomain, alias_name) DO UPDATE SET target_model=excluded.target_model`,
		alias.Subdomain, alias.AliasName, alias.TargetModel)
	return err
}

func (d *DB) DeleteAIRouterAlias(subdomain string, id int64) error {
	_, err := d.conn.Exec(`DELETE FROM ai_router_aliases WHERE id=? AND subdomain=?`, id, subdomain)
	return err
}

func (d *DB) ResolveAIRouterAlias(subdomain, modelName string) string {
	var target string
	err := d.conn.QueryRow(`SELECT target_model FROM ai_router_aliases WHERE subdomain=? AND alias_name=?`, subdomain, modelName).Scan(&target)
	if err != nil {
		return modelName // no alias, return as-is
	}
	return target
}
