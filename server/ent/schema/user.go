package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("openid").
			MaxLen(128).
			Optional().
			Nillable().
			Comment("微信 openid"),
		field.String("phone").
			MaxLen(20).
			Optional().
			Nillable().
			Comment("手机号（H5登录）"),
		field.String("password_hash").
			MaxLen(128).
			Optional().
			Nillable().
			Comment("bcrypt 密码哈希（H5登录）"),
		field.String("nickname").
			MaxLen(32).
			NotEmpty(),
		field.Text("avatar").
			Comment("头像 URL"),
		field.Int("level").
			Default(1).
			Min(1),
		field.Int64("exp").
			Default(0).
			Min(0),
		field.Int64("coins").
			Default(500).
			Min(0),
		field.Int("diamonds").
			Default(0).
			Min(0),
		field.Int("stamina").
			Default(100).
			Min(0),
		field.Int("max_stamina").
			Default(100).
			Min(1),
		field.Time("stamina_updated_at").
			Default(time.Now).
			Comment("体力最后变动时间，用于按需恢复计算"),
		field.Int("friendship_points").
			Default(0).
			Min(0),
		field.Bool("is_first_charge").
			Default(false),
		field.Time("subscription_expires_at").
			Optional().
			Nillable().
			Comment("月卡到期时间"),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// 1:1 owned entities
		edge.To("farm", Farm.Type).Unique(),
		edge.To("role", Role.Type).Unique(),
		edge.To("subscription", Subscription.Type).Unique(),
		edge.To("village_member", VillageMember.Type).Unique(),

		// 1:N owned entities
		edge.To("inventory_items", InventoryItem.Type),
		edge.To("notifications", Notification.Type),
		edge.To("payment_orders", PaymentOrder.Type),
		edge.To("activity_logs", ActivityLog.Type),
		edge.To("trade_orders", TradeOrder.Type),
		edge.To("daily_checkins", DailyCheckin.Type),
		edge.To("tutorial_progresses", TutorialProgress.Type),
		edge.To("season_scores", SeasonScore.Type),
		edge.To("season_task_progresses", SeasonTaskProgress.Type),
		edge.To("contributions", VillageProjectContribution.Type),

		// Activity interactions
		edge.To("activity_likes", ActivityLike.Type),
		edge.To("activity_comments", ActivityComment.Type),

		// Social: bidirectional symmetric edges (user_a < user_b)
		edge.To("relationships_as_a", Relationship.Type),
		edge.To("relationships_as_b", Relationship.Type),
		edge.To("friends_as_a", Friend.Type),
		edge.To("friends_as_b", Friend.Type),

		// Friend requests
		edge.To("sent_friend_requests", FriendRequest.Type),
		edge.To("received_friend_requests", FriendRequest.Type),

		// Trade participation
		edge.To("purchases", TradeTransaction.Type),
		edge.To("sales", TradeTransaction.Type),

		// Help & gifts
		edge.To("help_requests_sent", HelpRequest.Type),
		edge.To("help_requests_received", HelpRequest.Type),
		edge.To("gifts_sent", Gift.Type),
		edge.To("gifts_received", Gift.Type),

		// Chat logs (three roles: user_a, user_b, speaker)
		edge.To("chat_logs_as_a", ChatLog.Type),
		edge.To("chat_logs_as_b", ChatLog.Type),
		edge.To("chat_logs_spoken", ChatLog.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("openid").Unique().StorageKey("idx_users_openid"),
		index.Fields("phone").Unique().StorageKey("idx_users_phone"),
	}
}
