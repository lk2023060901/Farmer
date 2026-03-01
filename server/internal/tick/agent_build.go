package tick

import (
	"context"
	"log"
	"time"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	entrole "github.com/liukai/farmer/server/ent/role"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
	entvp "github.com/liukai/farmer/server/ent/villageproject"
	entvpc "github.com/liukai/farmer/server/ent/villageprojectcontribution"
)

// buildLevelThresholds mirrors the village.go handler thresholds for level-up.
// Index i = cumulative contribution required to reach level i+2.
var buildLevelThresholds = [5]int64{500, 2000, 5000, 15000, 50000}

const (
	maxDailyBuilds    = 2
	autoContributeQty = int64(5) // units contributed per auto-build action
)

// runAgentBuilds processes auto village co-building for agents in one tick pass.
// Only agents with strategy_social >= 4 participate.
func (e *Engine) runAgentBuilds(ctx context.Context) {
	agents, err := e.db.Role.Query().
		Where(entrole.StrategySocialGTE(4)).
		All(ctx)
	if err != nil {
		log.Printf("[tick/build] query agents: %v", err)
		return
	}
	for _, ag := range agents {
		e.maybeContributeToProject(ctx, ag)
	}
}

// maybeContributeToProject has the agent contribute resources to an active village project.
func (e *Engine) maybeContributeToProject(ctx context.Context, ag *ent.Role) {
	today := time.Now().Truncate(24 * time.Hour)

	// Count contributions already made today.
	todayBuilds, err := e.db.VillageProjectContribution.Query().
		Where(
			entvpc.UserID(ag.UserID),
			entvpc.CreatedAtGTE(today),
		).
		Count(ctx)
	if err != nil || todayBuilds >= maxDailyBuilds {
		return
	}

	// Find user's village.
	mem, err := e.db.VillageMember.Query().
		Where(entvm.UserID(ag.UserID)).
		Only(ctx)
	if err != nil {
		return
	}

	// Find an active village project.
	project, err := e.db.VillageProject.Query().
		Where(entvp.VillageID(mem.VillageID), entvp.Status("active")).
		First(ctx)
	if err != nil || project == nil {
		return
	}

	// Find the first incomplete requirement that we can help with.
	reqs := project.Requirements
	contributed := false
	for i := range reqs {
		r := &reqs[i]
		if r.Current >= r.Required {
			continue // already fulfilled
		}
		remaining := r.Required - r.Current
		qty := autoContributeQty
		if qty > remaining {
			qty = remaining
		}

		switch r.Type {
		case "material":
			if r.ItemID == "" {
				continue
			}
			// Check and deduct inventory.
			inv, err := e.db.InventoryItem.Query().
				Where(entinv.UserID(ag.UserID), entinv.ItemID(r.ItemID)).
				Only(ctx)
			if err != nil || inv.Quantity < qty {
				continue
			}
			if err := e.db.InventoryItem.UpdateOne(inv).
				AddQuantity(-qty).
				Exec(ctx); err != nil {
				continue
			}
			r.Current += qty
			contributed = true

		case "coins":
			// Deduct coins from user account.
			u, err := e.db.User.Get(ctx, ag.UserID)
			if err != nil || u.Coins < qty {
				continue
			}
			if err := e.db.User.UpdateOneID(ag.UserID).
				AddCoins(-qty).
				Exec(ctx); err != nil {
				continue
			}
			r.Current += qty
			contributed = true
		}

		if contributed {
			break
		}
	}

	if !contributed {
		return
	}

	// Check for project completion.
	allDone := true
	for _, r := range reqs {
		if r.Current < r.Required {
			allDone = false
			break
		}
	}
	newStatus := "active"
	if allDone {
		newStatus = "completed"
	}

	// Save updated requirements + status.
	upd := e.db.VillageProject.UpdateOne(project).
		SetRequirements(reqs)
	if allDone {
		now := time.Now()
		upd = upd.SetStatus(newStatus).SetCompletedAt(now)
	}
	if err := upd.Exec(ctx); err != nil {
		log.Printf("[tick/build] update project %s: %v", project.ID, err)
		return
	}

	// Record contribution row.
	if err := e.db.VillageProjectContribution.Create().
		SetProjectID(project.ID).
		SetUserID(ag.UserID).
		SetResourceType("agent_auto").
		SetQuantity(autoContributeQty).
		Exec(ctx); err != nil {
		log.Printf("[tick/build] record contribution: %v", err)
	}

	// Increment member + village contribution counters.
	_ = e.db.VillageMember.UpdateOne(mem).
		AddContribution(autoContributeQty).
		Exec(ctx)

	village, err := e.db.Village.Get(ctx, mem.VillageID)
	if err != nil {
		return
	}
	newContrib := village.Contribution + autoContributeQty
	newLevel := village.Level
	for newLevel < len(buildLevelThresholds) && newContrib >= buildLevelThresholds[newLevel-1] {
		newLevel++
	}
	_ = e.db.Village.UpdateOne(village).
		SetContribution(newContrib).
		SetLevel(newLevel).
		Exec(ctx)

	log.Printf("[tick/build] agent %s contributed %d to project %s", ag.Name, autoContributeQty, project.ID)
}
