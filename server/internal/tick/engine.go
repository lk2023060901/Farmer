// Package tick implements the global farm tick engine.
//
// Every TickInterval the engine scans every farm in the database and:
//  1. Advances crop growth stages (seedling → growing → mature → withered)
//  2. Runs the AI Agent rules engine to auto-harvest/plant/water
package tick

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/liukai/farmer/server/ent"
	entfarm "github.com/liukai/farmer/server/ent/farm"
	entrole "github.com/liukai/farmer/server/ent/role"
	entschema "github.com/liukai/farmer/server/ent/schema"
	entuser "github.com/liukai/farmer/server/ent/user"
	"github.com/liukai/farmer/server/internal/pathfinding"
	"github.com/liukai/farmer/server/internal/role"
	"github.com/liukai/farmer/server/internal/service"
	"github.com/liukai/farmer/server/internal/ws"
)

// TickInterval controls how often the engine runs.
const TickInterval = 60 * time.Second

// Engine processes all farms periodically and drives autonomous role movement.
type Engine struct {
	db   *ent.Client
	hub  *ws.Hub
	Grid *pathfinding.SpatialGrid // nine-grid AOI index, updated every movement tick
}

// New creates a new Engine.
func New(db *ent.Client, hub *ws.Hub) *Engine {
	return &Engine{db: db, hub: hub, Grid: pathfinding.NewSpatialGrid()}
}

// Start launches the tick loop in a background goroutine.
// The loop runs until ctx is cancelled.
func (e *Engine) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(TickInterval)
		defer ticker.Stop()
		log.Println("[tick] engine started, interval=", TickInterval)
		for {
			select {
			case <-ctx.Done():
				log.Println("[tick] engine stopped")
				return
			case <-ticker.C:
				e.runTick(ctx)
			}
		}
	}()
}

// tickConcurrency controls how many farms are processed in parallel.
const tickConcurrency = 10

// runTick processes all farms in a single pass.
// T-094: Only loads "active" farms — those ticked recently or with planted crops.
// This avoids full table scans as the inactive-farm ratio grows over time.
func (e *Engine) runTick(ctx context.Context) {
	// Only process farms active in the last 3 days (have recently been ticked or have crops).
	cutoff := time.Now().AddDate(0, 0, -3)
	farms, err := e.db.Farm.Query().
		Where(entfarm.LastTickAtGTE(cutoff)).
		All(ctx)
	if err != nil {
		log.Printf("[tick] query farms: %v", err)
		return
	}

	// T-094: Process farms concurrently with a bounded semaphore.
	type semaphore chan struct{}
	sem := make(semaphore, tickConcurrency)
	updated := 0

	for _, farm := range farms {
		// Fast-path: skip farms with all-empty plots (no crops to tick).
		hasActiveCrop := false
		for _, p := range farm.Plots {
			if p.Type == "planted" || p.Type == "mature" {
				hasActiveCrop = true
				break
			}
		}
		if !hasActiveCrop {
			continue
		}

		sem <- struct{}{} // acquire
		go func(f *ent.Farm) {
			defer func() { <-sem }() // release
			if changed := tickFarm(f.Plots); changed {
				_, err := e.db.Farm.UpdateOne(f).
					SetPlots(f.Plots).
					SetLastTickAt(time.Now()).
					Save(ctx)
				if err != nil {
					log.Printf("[tick] save farm %s: %v", f.ID, err)
					return
				}
				updated++
			}
			// Reload and run agent actions
			reloaded, err := e.db.Farm.Get(ctx, f.ID)
			if err == nil {
				e.runAgentActions(ctx, reloaded)
			}
		}(farm)
	}
	// Drain semaphore to wait for all goroutines.
	for i := 0; i < tickConcurrency; i++ {
		sem <- struct{}{}
	}

	if updated > 0 {
		log.Printf("[tick] updated %d/%d active farms", updated, len(farms))
	}

	// Social visits: each agent may visit a village neighbour
	e.runSocialVisits(ctx)

	// Auto-trade: agents with high trade strategy create surplus listings
	e.runAgentTrades(ctx)

	// Auto-build: agents with high social strategy contribute to village projects
	e.runAgentBuilds(ctx)

	// Personality evolution: drift agent traits toward strategy alignment
	e.runPersonalityEvolution(ctx)

	// Random events: probabilistic daily events (storms, merchants, etc.)
	e.runRandomEvents(ctx)
}

// runAgentActions fetches the farm owner's agent, runs the rules engine,
// and executes the resulting actions (harvest / plant / water).
func (e *Engine) runAgentActions(ctx context.Context, farm *ent.Farm) {
	ag, err := e.db.Role.Query().
		Where(entrole.UserID(farm.OwnerID)).
		Only(ctx)
	if err != nil {
		return // no agent configured yet
	}

	u, err := e.db.User.Query().Where(entuser.ID(farm.OwnerID)).Only(ctx)
	if err != nil {
		return
	}

	params := role.Params{
		StrategyManagement: ag.StrategyManagement,
		StrategyPlanting:   ag.StrategyPlanting,
		StrategyTrade:      ag.StrategyTrade,
		StrategyResource:   ag.StrategyResource,
		Extroversion:       ag.Extroversion,
		Generosity:         ag.Generosity,
		Adventure:          ag.Adventure,
		UserLevel:          u.Level,
		Stamina:            u.Stamina,
	}

	actions := role.Decide(farm.Plots, params)
	if len(actions) == 0 {
		return
	}

	plots := farm.Plots
	plotsChanged := false
	staminaSpent := 0

	for _, a := range actions {
		idx := a.Y*8 + a.X
		if idx < 0 || idx >= len(plots) {
			continue
		}

		switch a.Type {
		case role.ActionHarvest:
			if plots[idx].Type != "mature" {
				continue
			}
			crop := service.GetCrop(plots[idx].CropID)
			if crop == nil {
				continue
			}
			plots[idx] = entschema.PlotState{X: a.X, Y: a.Y, Type: "empty"}
			plotsChanged = true

			// Add to inventory
			if err := service.AddToInventory(ctx, e.db, farm.OwnerID, crop.ID, "crop", 1); err != nil {
				log.Printf("[tick/agent] inventory err: %v", err)
			}
			// Grant coins + exp
			oldLevel := u.Level
			newExp := u.Exp + int64(crop.ExpReward)
			newLevel := CalcLevel(newExp)
			upd := e.db.User.UpdateOne(u).
				AddCoins(int64(crop.CoinReward)).
				AddExp(int64(crop.ExpReward))
			if newLevel > oldLevel {
				upd = upd.SetLevel(newLevel)
			}
			u, _ = upd.Save(ctx)

		case role.ActionPlant:
			p := plots[idx]
			if p.Type != "empty" && p.Type != "tilled" {
				continue
			}
			crop := service.GetCrop(a.CropID)
			if crop == nil || crop.UnlockLevel > u.Level {
				continue
			}
			plots[idx] = entschema.PlotState{
				X:         a.X,
				Y:         a.Y,
				Type:      "planted",
				CropID:    a.CropID,
				PlantedAt: time.Now().Format(time.RFC3339),
				Stage:     "seedling",
			}
			plotsChanged = true

		case role.ActionWater:
			p := &plots[idx]
			if p.Type != "planted" || p.WateredAt != "" {
				continue
			}
			if u.Stamina-staminaSpent < 2 {
				continue
			}
			p.WateredAt = time.Now().Format(time.RFC3339)
			staminaSpent += 2
			plotsChanged = true
		}
	}

	if staminaSpent > 0 {
		u, _ = e.db.User.UpdateOne(u).AddStamina(-staminaSpent).Save(ctx)
		_ = u
	}

	if plotsChanged {
		_, err = e.db.Farm.Query().
			Where(entfarm.ID(farm.ID)).
			Only(ctx)
		if err == nil {
			_, _ = e.db.Farm.UpdateOne(farm).SetPlots(plots).Save(ctx)
		}
	}
}

// tickFarm mutates the plots slice in-place.
// Returns true if any plot changed.
func tickFarm(plots []entschema.PlotState) bool {
	now := time.Now()
	changed := false

	for i := range plots {
		p := &plots[i]
		if p.Type != "planted" && p.Type != "mature" {
			continue
		}

		crop := service.GetCrop(p.CropID)
		if crop == nil {
			continue
		}

		plantedAt, err := time.Parse(time.RFC3339, p.PlantedAt)
		if err != nil {
			continue
		}

		elapsed := now.Sub(plantedAt)

		// Watering gives 20% growth acceleration
		effectiveDuration := crop.GrowDuration
		if p.WateredAt != "" {
			effectiveDuration = time.Duration(float64(crop.GrowDuration) * 0.8)
		}

		switch {
		case elapsed >= effectiveDuration*3:
			// Unattended 3× effective grow time → withered
			if p.Type != "withered" {
				p.Type = "withered"
				p.Stage = ""
				changed = true
			}
		case elapsed >= effectiveDuration:
			// Mature
			if p.Type != "mature" || p.Stage != "mature" {
				p.Type = "mature"
				p.Stage = "mature"
				changed = true
			}
		case elapsed >= effectiveDuration/2:
			// Second half of growth → growing
			if p.Stage != "growing" {
				p.Stage = "growing"
				changed = true
			}
		default:
			// First half → seedling (initial state, no change needed)
		}
	}
	return changed
}

// CalcLevel returns the player level for the given total exp.
// Formula: level = 1 + floor(sqrt(exp / 100))
func CalcLevel(exp int64) int {
	if exp <= 0 {
		return 1
	}
	return 1 + int(math.Sqrt(float64(exp)/100))
}
