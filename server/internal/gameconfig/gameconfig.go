// Package gameconfig loads static game configuration files embedded at compile time.
// Only fields used by server business logic are defined here.
package gameconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed items.json
var itemsJSON []byte

//go:embed roles.json
var rolesJSON []byte

//go:embed global.json
var globalJSON []byte

//go:embed maps.json
var mapsJSON []byte

// ── Server-side field definitions ────────────────────────────────────────────
// These structs only contain fields the server actually uses.
// Display fields (name, description, icon …) are client-only concerns.

// Item holds the server-relevant subset of an items.json entry.
type Item struct {
	ID         string `json:"id"`
	Category   string `json:"category"`   // becomes item_type in inventory table
	Tradeable  bool   `json:"tradeable"`  // trade validation
	Sellable   bool   `json:"sellable"`   // sell validation
	SellPrice  int    `json:"sell_price"` // sell transaction amount
	BuyPrice   int    `json:"buy_price"`  // shop purchase amount
	StackLimit int    `json:"stack_limit"`
}

// Role holds the server-relevant subset of a roles.json entry.
// Used to initialise an Agent's name, avatar, and personality on first registration.
type Role struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Avatar             string `json:"avatar"`
	Extroversion       int    `json:"extroversion"`
	Generosity         int    `json:"generosity"`
	Adventure          int    `json:"adventure"`
	StrategyManagement int    `json:"strategy_management"`
	StrategyPlanting   int    `json:"strategy_planting"`
	StrategySocial     int    `json:"strategy_social"`
	StrategyTrade      int    `json:"strategy_trade"`
	StrategyResource   int    `json:"strategy_resource"`
}

// DefaultItem is one entry in the new_user_defaults starter item list.
type DefaultItem struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// GlobalConfig is global.json — bootstraps coins, diamonds, inventory, and role on registration.
type GlobalConfig struct {
	Coins    int64         `json:"coins"`
	Diamonds int           `json:"diamonds"`
	Items    []DefaultItem `json:"items"`
	RoleID   int           `json:"role_id"`
}

// ── Config ────────────────────────────────────────────────────────────────────

// SpawnPoint holds the spawn tile coordinates for a map.
type SpawnPoint struct {
	TileX int `json:"tile_x"`
	TileY int `json:"tile_y"`
}

// mapEntry is the server-relevant subset of a maps.json entry.
type mapEntry struct {
	ID    string     `json:"id"`
	Spawn SpawnPoint `json:"spawn"`
}

// Config is the parsed, indexed view of all embedded game config files.
// Create once at startup via Load() and share the pointer.
type Config struct {
	Global     GlobalConfig
	itemsByID  map[string]*Item
	rolesByID  map[int]*Role
	spawnsByID map[string]SpawnPoint
}

// Load parses all embedded JSON files. Call once at startup.
func Load() (*Config, error) {
	var itemsWrapper struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(stripComments(itemsJSON), &itemsWrapper); err != nil {
		return nil, fmt.Errorf("parse items.json: %w", err)
	}

	var rolesWrapper struct {
		Roles []Role `json:"roles"`
	}
	if err := json.Unmarshal(rolesJSON, &rolesWrapper); err != nil {
		return nil, fmt.Errorf("parse roles.json: %w", err)
	}

	var global GlobalConfig
	if err := json.Unmarshal(globalJSON, &global); err != nil {
		return nil, fmt.Errorf("parse global.json: %w", err)
	}

	var mapsWrapper struct {
		Maps []mapEntry `json:"maps"`
	}
	if err := json.Unmarshal(mapsJSON, &mapsWrapper); err != nil {
		return nil, fmt.Errorf("parse maps.json: %w", err)
	}

	cfg := &Config{
		Global:     global,
		itemsByID:  make(map[string]*Item, len(itemsWrapper.Items)),
		rolesByID:  make(map[int]*Role, len(rolesWrapper.Roles)),
		spawnsByID: make(map[string]SpawnPoint, len(mapsWrapper.Maps)),
	}
	for i := range itemsWrapper.Items {
		cfg.itemsByID[itemsWrapper.Items[i].ID] = &itemsWrapper.Items[i]
	}
	for i := range rolesWrapper.Roles {
		cfg.rolesByID[rolesWrapper.Roles[i].ID] = &rolesWrapper.Roles[i]
	}
	for _, m := range mapsWrapper.Maps {
		cfg.spawnsByID[m.ID] = m.Spawn
	}
	return cfg, nil
}

// GetItem returns the Item with the given ID, or nil if not found.
func (c *Config) GetItem(id string) *Item { return c.itemsByID[id] }

// GetRole returns the Role with the given ID, or nil if not found.
func (c *Config) GetRole(id int) *Role { return c.rolesByID[id] }

// Spawn returns the spawn point for the given map ID from maps.json.
func (c *Config) Spawn(mapID string) SpawnPoint {
	if s, ok := c.spawnsByID[mapID]; ok {
		return SpawnPoint{TileX: s.TileX, TileY: s.TileY}
	}
	return SpawnPoint{TileX: 0, TileY: 0}
}

// stripComments removes // single-line comments so encoding/json can parse the files.
func stripComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inStr := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if ch == '"' {
			inStr = !inStr
		}
		if !inStr && ch == '/' && i+1 < len(src) && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		out = append(out, ch)
	}
	return out
}
