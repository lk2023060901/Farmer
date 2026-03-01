// Package role contains the AI farming-agent decision engine.
//
// The rules engine is a pure function: given farm state and agent parameters,
// it returns a list of recommended actions. This makes it trivially testable
// and usable from both the tick engine (automated) and the API (preview).
package role

import (
	"fmt"
	"sort"

	entschema "github.com/liukai/farmer/server/ent/schema"
	"github.com/liukai/farmer/server/internal/service"
)

// ActionType enumerates the kinds of actions an agent can decide on.
type ActionType string

const (
	ActionHarvest ActionType = "harvest"
	ActionPlant   ActionType = "plant"
	ActionWater   ActionType = "water"
)

// Action is a single recommended farm operation.
type Action struct {
	Type   ActionType `json:"type"`
	X      int        `json:"x"`
	Y      int        `json:"y"`
	CropID string     `json:"cropId,omitempty"` // only for plant
}

// Params holds the agent's strategy + personality dimensions.
type Params struct {
	// Strategy (1-5)
	StrategyManagement int
	StrategyPlanting   int
	StrategyTrade      int
	StrategyResource   int
	// Personality (1-10)
	Extroversion int
	Generosity   int
	Adventure    int
	// User state
	UserLevel int
	Stamina   int
}

// Decide returns an ordered list of actions the agent should take this tick.
// Actions are ordered: harvest first, then plant, then water.
// Returns an empty (non-nil) slice when there is nothing to do.
func Decide(plots []entschema.PlotState, p Params) []Action {
	actions := make([]Action, 0)

	// ── 1. Harvest all mature plots ──────────────────────────────────────────
	for _, plot := range plots {
		if plot.Type == "mature" {
			actions = append(actions, Action{Type: ActionHarvest, X: plot.X, Y: plot.Y})
		}
	}

	// ── 2. Plant empty/tilled plots ──────────────────────────────────────────
	preferredCrop := chooseCrop(p)
	for _, plot := range plots {
		if plot.Type == "empty" || plot.Type == "tilled" {
			if preferredCrop != nil {
				actions = append(actions, Action{
					Type:   ActionPlant,
					X:      plot.X,
					Y:      plot.Y,
					CropID: preferredCrop.ID,
				})
			}
		}
	}

	// ── 3. Water unwatered growing plots (if stamina allows) ─────────────────
	// Only water if strategy_management >= 3 (values effort)
	if p.StrategyManagement >= 3 {
		staminaLeft := p.Stamina
		for _, plot := range plots {
			if plot.Type == "planted" && plot.WateredAt == "" {
				if staminaLeft >= 2 {
					actions = append(actions, Action{Type: ActionWater, X: plot.X, Y: plot.Y})
					staminaLeft -= 2
				}
			}
		}
	}

	return actions
}

// Explain returns a human-readable summary of what the agent will do and why.
func Explain(plots []entschema.PlotState, p Params) string {
	actions := Decide(plots, p)
	harvests, plants, waters := 0, 0, 0
	crop := chooseCrop(p)
	for _, a := range actions {
		switch a.Type {
		case ActionHarvest:
			harvests++
		case ActionPlant:
			plants++
		case ActionWater:
			waters++
		}
	}

	cropName := "无"
	if crop != nil {
		cropName = crop.Name
	}

	return fmt.Sprintf(
		"今日计划：收获 %d 块，种植 %d 块（%s），浇水 %d 块。"+
			"策略：种植偏好 %d/5，经营风格 %d/5。",
		harvests, plants, cropName, waters,
		p.StrategyPlanting, p.StrategyManagement,
	)
}

// chooseCrop picks the best crop for the agent based on strategy + level.
// StrategyPlanting: 1-2 = quick/cheap, 3 = balanced, 4-5 = high-value/long
func chooseCrop(p Params) *service.CropDef {
	all := service.AllCrops()

	// Filter by unlock level
	var available []*service.CropDef
	for _, c := range all {
		if c.UnlockLevel <= p.UserLevel {
			available = append(available, c)
		}
	}
	if len(available) == 0 {
		return nil
	}

	// Score each crop
	type scored struct {
		crop  *service.CropDef
		score float64
	}
	var candidates []scored
	for _, c := range available {
		candidates = append(candidates, scored{crop: c, score: scoreCrop(c, p)})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].crop
}

// scoreCrop assigns a fitness score to a crop given agent preferences.
func scoreCrop(c *service.CropDef, p Params) float64 {
	// coins-per-minute as baseline
	growMins := c.GrowDuration.Minutes()
	if growMins == 0 {
		growMins = 1
	}
	coinsPerMin := float64(c.CoinReward) / growMins

	// strategy_planting: 1-2 → prefer speed, 4-5 → prefer high total reward
	speedWeight := float64(6-p.StrategyPlanting) / 5.0  // [0.2, 1.0]
	rewardWeight := float64(p.StrategyPlanting) / 5.0   // [0.2, 1.0]

	// adventure: high → try higher unlock-level crops
	adventureBonus := float64(p.Adventure) * float64(c.UnlockLevel) * 0.05

	return speedWeight*coinsPerMin + rewardWeight*float64(c.CoinReward) + adventureBonus
}
