package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/liukai/farmer/server/ent"
)

// CreateActivityLog inserts an activity log entry for a user.
// activityType examples: "harvest", "level_up", "social_visit", "agent_plant", "agent_harvest"
// content is the human-readable description shown in the feed.
// meta holds optional extra data (cropId, yield, targetUserId, etc.).
func CreateActivityLog(
	ctx context.Context,
	db *ent.Client,
	userID uuid.UUID,
	activityType string,
	content string,
	meta map[string]any,
) error {
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := db.ActivityLog.Create().
		SetUserID(userID).
		SetType(activityType).
		SetContent(content).
		SetMeta(meta).
		Save(ctx)
	return err
}
