package tick

import (
	"bytes"
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entrole "github.com/liukai/farmer/server/ent/role"
	entfarm "github.com/liukai/farmer/server/ent/farm"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
	"github.com/liukai/farmer/server/internal/dialogue"
	"github.com/liukai/farmer/server/internal/service"
)

const maxDailyVisits = 3

// runSocialVisits processes social visits for all agents in one tick pass.
// It is called once per tick from runTick.
func (e *Engine) runSocialVisits(ctx context.Context) {
	agents, err := e.db.Role.Query().All(ctx)
	if err != nil {
		log.Printf("[tick/social] query agents: %v", err)
		return
	}
	for _, ag := range agents {
		e.maybeSocialVisit(ctx, ag)
	}
}

// maybeSocialVisit decides (probabilistically) whether this agent visits a neighbour.
func (e *Engine) maybeSocialVisit(ctx context.Context, ag *ent.Role) {
	// Probability based on extroversion (1-10) and strategy_social (1-5).
	// Range: 5% (extroversion=1, social=1) to 40% (extroversion=10, social=5)
	prob := float64(ag.Extroversion+ag.StrategySocial) / 75.0
	if rand.Float64() >= prob {
		return
	}

	// Reset daily counter if it's a new day
	today := time.Now().Truncate(24 * time.Hour)
	resetNeeded := ag.DailySocialDate == nil || ag.DailySocialDate.Truncate(24*time.Hour).Before(today)
	if resetNeeded {
		ag, _ = e.db.Role.UpdateOne(ag).
			SetDailySocialCount(0).
			SetDailySocialDate(time.Now()).
			Save(ctx)
		if ag == nil {
			return
		}
	}

	if ag.DailySocialCount >= maxDailyVisits {
		return
	}

	// Find which village this user belongs to
	mem, err := e.db.VillageMember.Query().
		Where(entvm.UserID(ag.UserID)).
		Only(ctx)
	if err != nil {
		return // not in a village
	}

	// Find all other members in the same village
	others, err := e.db.VillageMember.Query().
		Where(entvm.VillageID(mem.VillageID), entvm.UserIDNEQ(ag.UserID)).
		All(ctx)
	if err != nil || len(others) == 0 {
		return
	}

	// Pick a random target
	target := others[rand.Intn(len(others))]
	targetID := target.UserID

	// Retrieve names for dialogue variables
	targetAgent, err := e.db.Role.Query().
		Where(entrole.UserID(targetID)).
		Only(ctx)
	if err != nil {
		return
	}

	// Get current relationship level
	rel, err := service.GetOrCreateRelationship(ctx, e.db, ag.UserID, targetID)
	if err != nil {
		return
	}

	// Choose a recent crop from the caller's farm (optional: blank if unavailable)
	cropName := lastCropName(ctx, e.db, ag.UserID)

	vars := dialogue.Vars{
		AgentName:  ag.Name,
		TargetName: targetAgent.Name,
		CropName:   cropName,
		Season:     "spring", // TODO: use real season from season service
	}

	lines := dialogue.Generate("visit", rel.Level, ag.Extroversion, vars)

	// Persist chat lines
	aID, bID := normaliseUUIDs(ag.UserID, targetID)
	for _, line := range lines {
		speakerID := ag.UserID
		if line.SpeakerRole == "target" {
			speakerID = targetID
		}
		if err := e.db.ChatLog.Create().
			SetUserAID(aID).
			SetUserBID(bID).
			SetSpeakerUserID(speakerID).
			SetScene("visit").
			SetContent(line.Text).
			Exec(ctx); err != nil {
			log.Printf("[tick/social] chatlog insert: %v", err)
		}
	}

	// Observation comment: the visiting agent remarks on what they see in the farm.
	if obs := farmObservationComment(ctx, e.db, targetID, ag.Name); obs != "" {
		_ = e.db.ChatLog.Create().
			SetUserAID(aID).
			SetUserBID(bID).
			SetSpeakerUserID(ag.UserID).
			SetScene("visit_observation").
			SetContent(obs).
			Exec(ctx)
	}

	// Increment affinity: +5 for visit, +3 per dialogue turn (up to one bonus)
	if _, err := service.AddAffinity(ctx, e.db, ag.UserID, targetID, service.AffinityVisit); err != nil {
		log.Printf("[tick/social] affinity visit: %v", err)
	}
	if len(lines) >= 3 {
		if _, err := service.AddAffinity(ctx, e.db, ag.UserID, targetID, service.AffinityChatMessage); err != nil {
			log.Printf("[tick/social] affinity chat: %v", err)
		}
	}

	// Increment daily counter
	_, _ = e.db.Role.UpdateOne(ag).AddDailySocialCount(1).Save(ctx)

	log.Printf("[tick/social] %s visited %s (%d lines)", ag.Name, targetAgent.Name, len(lines))
}

// lastCropName returns the name of a crop found on the user's farm, or "作物".
func lastCropName(ctx context.Context, db *ent.Client, userID uuid.UUID) string {
	farm, err := db.Farm.Query().
		Where(entfarm.OwnerID(userID)).
		Only(ctx)
	if err != nil {
		return "作物"
	}
	for _, p := range farm.Plots {
		if p.CropID != "" {
			if cd := service.GetCrop(p.CropID); cd != nil {
				return cd.Name
			}
		}
	}
	return "作物"
}

// farmObservationComment generates a short observation comment an agent makes
// about the target user's farm (added as an extra chat line after the visit).
func farmObservationComment(ctx context.Context, db *ent.Client, targetID uuid.UUID, agentName string) string {
	farm, err := db.Farm.Query().Where(entfarm.OwnerID(targetID)).Only(ctx)
	if err != nil {
		return ""
	}
	mature, planted, empty := 0, 0, 0
	for _, p := range farm.Plots {
		switch p.Type {
		case "mature":
			mature++
		case "planted":
			planted++
		case "empty", "tilled":
			empty++
		}
	}
	switch {
	case mature >= 4:
		return agentName + "：你的农场大丰收了，赶紧收割吧！"
	case planted >= 6:
		return agentName + "：你的农场生机勃勃，农作物长势喜人！"
	case empty >= 6:
		return agentName + "：你的农场还有好多空地，多种些作物吧！"
	default:
		return agentName + "：你的农场打理得真不错！"
	}
}

// normaliseUUIDs returns (a, b) with a <= b (same canonical order as relationship service).
func normaliseUUIDs(x, y uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(x[:], y[:]) <= 0 {
		return x, y
	}
	return y, x
}
