package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Parental Control methods
func (d *DB) GetParentalConfig(subdomain string) (*ParentalConfig, error) {
	row := d.conn.QueryRow(`
		SELECT subdomain, enabled, block_adult, block_gambling, block_malware, block_ads, block_social, block_gaming, enforce_safesearch, block_mode, custom_blocked, custom_allowed, updated_at
		FROM parental_configs WHERE subdomain = ?
	`, subdomain)

	var p ParentalConfig
	var customBlockedRaw, customAllowedRaw string
	var enabled, bAdult, bGambling, bMalware, bAds, bSocial, bGaming, safeSearch int

	err := row.Scan(
		&p.Subdomain, &enabled, &bAdult, &bGambling, &bMalware, &bAds, &bSocial, &bGaming,
		&safeSearch, &p.BlockMode, &customBlockedRaw, &customAllowedRaw, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default config
			return &ParentalConfig{
				Subdomain:         subdomain,
				Enabled:           true,
				BlockAdult:        true,
				BlockGambling:     true,
				BlockMalware:      true,
				BlockAds:          true,
				BlockSocial:       false,
				BlockGaming:       false,
				EnforceSafeSearch: true,
				BlockMode:         "0.0.0.0",
				CustomBlocked:     []string{},
				CustomAllowed:     []string{},
				UpdatedAt:         time.Now(),
			}, nil
		}
		return nil, err
	}

	p.Enabled = enabled == 1
	p.BlockAdult = bAdult == 1
	p.BlockGambling = bGambling == 1
	p.BlockMalware = bMalware == 1
	p.BlockAds = bAds == 1
	p.BlockSocial = bSocial == 1
	p.BlockGaming = bGaming == 1
	p.EnforceSafeSearch = safeSearch == 1
	p.CustomBlocked = parseStringSlice(customBlockedRaw)
	p.CustomAllowed = parseStringSlice(customAllowedRaw)

	return &p, nil
}

func (d *DB) SaveParentalConfig(p *ParentalConfig) error {
	blockedJSON, _ := json.Marshal(p.CustomBlocked)
	allowedJSON, _ := json.Marshal(p.CustomAllowed)

	toBoolInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	_, err := d.conn.Exec(`
		INSERT INTO parental_configs (
			subdomain, enabled, block_adult, block_gambling, block_malware, block_ads, block_social, block_gaming, enforce_safesearch, block_mode, custom_blocked, custom_allowed, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(subdomain) DO UPDATE SET
			enabled = excluded.enabled,
			block_adult = excluded.block_adult,
			block_gambling = excluded.block_gambling,
			block_malware = excluded.block_malware,
			block_ads = excluded.block_ads,
			block_social = excluded.block_social,
			block_gaming = excluded.block_gaming,
			enforce_safesearch = excluded.enforce_safesearch,
			block_mode = excluded.block_mode,
			custom_blocked = excluded.custom_blocked,
			custom_allowed = excluded.custom_allowed,
			updated_at = CURRENT_TIMESTAMP
	`,
		p.Subdomain,
		toBoolInt(p.Enabled),
		toBoolInt(p.BlockAdult),
		toBoolInt(p.BlockGambling),
		toBoolInt(p.BlockMalware),
		toBoolInt(p.BlockAds),
		toBoolInt(p.BlockSocial),
		toBoolInt(p.BlockGaming),
		toBoolInt(p.EnforceSafeSearch),
		p.BlockMode,
		string(blockedJSON),
		string(allowedJSON),
	)
	return err
}
