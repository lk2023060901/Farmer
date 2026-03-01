// Package service provides domain logic for social relationships.
package service

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entrel "github.com/liukai/farmer/server/ent/relationship"
)

// AffinityDelta constants (PRD §5.3)
const (
	AffinityVisit = 5
	AffinityChatMessage = 3
	AffinityGift  = 15 // midpoint of 10-20
	AffinityTrade = 5

	AffinityDecayDaily        = -1
	AffinityDecayInactive     = -2 // if last_interact > 3 days
	AffinityInactiveThreshold = 3 * 24 * time.Hour
)

// AffinityLevel thresholds → label mapping
func affinityLevel(affinity int) string {
	switch {
	case affinity >= 80:
		return "best_friend"
	case affinity >= 50:
		return "close_friend"
	case affinity >= 20:
		return "friend"
	case affinity > 0:
		return "acquaintance"
	default:
		return "stranger"
	}
}

// normalizeIDs returns (a, b) such that a < b (UUID byte-lexicographic order).
func normalizeIDs(x, y uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(x[:], y[:]) <= 0 {
		return x, y
	}
	return y, x
}

// GetOrCreateRelationship finds or creates the relationship row for the two users.
func GetOrCreateRelationship(ctx context.Context, db *ent.Client, u1, u2 uuid.UUID) (*ent.Relationship, error) {
	aID, bID := normalizeIDs(u1, u2)

	rel, err := db.Relationship.Query().
		Where(entrel.UserAID(aID), entrel.UserBID(bID)).
		Only(ctx)
	if err == nil {
		return rel, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}

	return db.Relationship.Create().
		SetUserAID(aID).
		SetUserBID(bID).
		SetAffinity(0).
		SetLevel("stranger").
		SetLastInteractAt(time.Now()).
		Save(ctx)
}

// AddAffinity increments affinity by delta (clamped 0-100) and recalculates level.
func AddAffinity(ctx context.Context, db *ent.Client, u1, u2 uuid.UUID, delta int) (*ent.Relationship, error) {
	rel, err := GetOrCreateRelationship(ctx, db, u1, u2)
	if err != nil {
		return nil, err
	}

	newAffinity := rel.Affinity + delta
	if newAffinity < 0 {
		newAffinity = 0
	}
	if newAffinity > 100 {
		newAffinity = 100
	}

	return db.Relationship.UpdateOne(rel).
		SetAffinity(newAffinity).
		SetLevel(affinityLevel(newAffinity)).
		SetLastInteractAt(time.Now()).
		Save(ctx)
}

// ListRelationships returns all relationships involving the given user.
func ListRelationships(ctx context.Context, db *ent.Client, userID uuid.UUID) ([]*ent.Relationship, error) {
	return db.Relationship.Query().
		Where(
			entrel.Or(
				entrel.UserAID(userID),
				entrel.UserBID(userID),
			),
		).
		Order(ent.Desc(entrel.FieldAffinity)).
		All(ctx)
}

// ApplyDailyDecay decrements affinity for all relationships by 1 (-2 if inactive > 3 days).
// Intended to be called once per day from the tick engine.
func ApplyDailyDecay(ctx context.Context, db *ent.Client) error {
	rels, err := db.Relationship.Query().
		Where(entrel.AffinityGT(0)).
		All(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, rel := range rels {
		decay := AffinityDecayDaily
		if now.Sub(rel.LastInteractAt) > AffinityInactiveThreshold {
			decay = AffinityDecayInactive
		}
		newAff := rel.Affinity + decay
		if newAff < 0 {
			newAff = 0
		}
		_, _ = db.Relationship.UpdateOne(rel).
			SetAffinity(newAff).
			SetLevel(affinityLevel(newAff)).
			Save(ctx)
	}
	return nil
}
