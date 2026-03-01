package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// VillageMember holds the schema definition for the VillageMember entity.
type VillageMember struct {
	ent.Schema
}

func (VillageMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("village_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}).
			Comment("一个用户只能加入一个村庄"),
		field.String("role").
			MaxLen(16).
			Default("member").
			Comment("member/elder/chief"),
		field.Int64("contribution").
			Default(0).
			Min(0).
			Comment("个人对本村总贡献"),
		field.Time("joined_at").
			Default(time.Now).
			Immutable(),
	}
}

func (VillageMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("village", Village.Type).
			Ref("members").
			Field("village_id").
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("village_member").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (VillageMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique().StorageKey("uidx_vm_user_id"),
		index.Fields("village_id").StorageKey("idx_vm_village_id"),
	}
}
