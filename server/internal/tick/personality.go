package tick

import (
	"context"
	"log"

	entrole "github.com/liukai/farmer/server/ent/role"
)

// runPersonalityEvolution drifts each agent's personality traits (Extroversion,
// Generosity, Adventure) one step toward the targets implied by the agent's
// strategy preferences.
//
// Drift rules (from PRD §Agent人格演化):
//
//   - targetExtroversion  = clamp(StrategySocial,                         1, 10)
//   - targetGenerosity    = clamp(StrategyResource,                        1, 10)
//   - targetAdventure     = clamp((StrategyTrade + StrategyPlanting) / 2,  1, 10)
//
// Each trait moves by at most ±1 per call.  When already at the target the
// value is unchanged.  This function is a no-op when all traits are already
// aligned with the current strategy values.
func (e *Engine) runPersonalityEvolution(ctx context.Context) {
	agents, err := e.db.Role.Query().All(ctx)
	if err != nil {
		log.Printf("[tick/personality] query agents: %v", err)
		return
	}

	updated := 0
	for _, ag := range agents {
		targetExt := clamp(ag.StrategySocial, 1, 10)
		targetGen := clamp(ag.StrategyResource, 1, 10)
		targetAdv := clamp((ag.StrategyTrade+ag.StrategyPlanting)/2, 1, 10)

		newExt := driftToward(ag.Extroversion, targetExt)
		newGen := driftToward(ag.Generosity, targetGen)
		newAdv := driftToward(ag.Adventure, targetAdv)

		// Skip DB write when nothing changed.
		if newExt == ag.Extroversion && newGen == ag.Generosity && newAdv == ag.Adventure {
			continue
		}

		_, err := e.db.Role.UpdateOne(ag).
			SetExtroversion(newExt).
			SetGenerosity(newGen).
			SetAdventure(newAdv).
			Save(ctx)
		if err != nil {
			log.Printf("[tick/personality] update agent %s: %v", ag.ID, err)
		} else {
			updated++
		}
	}

	if updated > 0 {
		log.Printf("[tick/personality] evolved %d/%d agents", updated, len(agents))
	}
}

// clamp returns v clamped to the inclusive range [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// driftToward returns current moved by exactly ±1 toward target.
// If current already equals target it is returned unchanged.
func driftToward(current, target int) int {
	if current < target {
		return current + 1
	}
	if current > target {
		return current - 1
	}
	return current
}

// Ensure the entrole import is used (the package is imported for its
// predicate functions elsewhere in the tick package; this blank import keeps
// go vet happy if no role predicate is needed directly in this file).
var _ = entrole.FieldExtroversion
