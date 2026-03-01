package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/liukai/farmer/server/ent/enttest"
	"github.com/liukai/farmer/server/internal/service"
	_ "github.com/mattn/go-sqlite3"
)

func TestAddAffinity(t *testing.T) {
	db := enttest.Open(t, "sqlite3", "file:ent_rel1?mode=memory&cache=shared&_fk=1")
	defer db.Close()

	ctx := context.Background()

	// Relationship schema has FK → users; create users first
	aID := uuid.New()
	bID := uuid.New()
	if _, err := db.User.Create().SetID(aID).SetNickname("A").SetAvatar("").Save(ctx); err != nil {
		t.Fatal("create user A:", err)
	}
	if _, err := db.User.Create().SetID(bID).SetNickname("B").SetAvatar("").Save(ctx); err != nil {
		t.Fatal("create user B:", err)
	}

	// First interaction: visit +5
	rel, err := service.AddAffinity(ctx, db, aID, bID, service.AffinityVisit)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Affinity != 5 {
		t.Fatalf("expected affinity 5, got %d", rel.Affinity)
	}
	if rel.Level != "acquaintance" {
		t.Fatalf("expected acquaintance, got %s", rel.Level)
	}

	// 3 more visits of +5 → total 20 → "friend"
	for i := 0; i < 3; i++ {
		rel, err = service.AddAffinity(ctx, db, aID, bID, service.AffinityVisit)
		if err != nil {
			t.Fatal(err)
		}
	}
	if rel.Affinity != 20 {
		t.Fatalf("expected affinity 20, got %d", rel.Affinity)
	}
	if rel.Level != "friend" {
		t.Fatalf("expected friend, got %s", rel.Level)
	}

	// normalizeIDs: (b, a) should return the same row
	rel2, err := service.GetOrCreateRelationship(ctx, db, bID, aID)
	if err != nil {
		t.Fatal(err)
	}
	if rel2.ID != rel.ID {
		t.Fatal("normalizeIDs broken: got different rows for (a,b) and (b,a)")
	}

	// ListRelationships for A should return exactly 1 row
	rels, err := service.ListRelationships(ctx, db, aID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(rels))
	}
}

func TestListRelationshipsEmpty(t *testing.T) {
	db := enttest.Open(t, "sqlite3", "file:ent_rel2?mode=memory&cache=shared&_fk=1")
	defer db.Close()

	ctx := context.Background()
	rels, err := service.ListRelationships(ctx, db, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 0 {
		t.Fatalf("expected 0 relationships, got %d", len(rels))
	}
}

func TestAffinityClampMax(t *testing.T) {
	db := enttest.Open(t, "sqlite3", "file:ent_rel3?mode=memory&cache=shared&_fk=1")
	defer db.Close()

	ctx := context.Background()
	aID, bID := uuid.New(), uuid.New()
	if _, err := db.User.Create().SetID(aID).SetNickname("A").SetAvatar("").Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.User.Create().SetID(bID).SetNickname("B").SetAvatar("").Save(ctx); err != nil {
		t.Fatal(err)
	}

	// Push well past 100 — should clamp at 100
	for i := 0; i < 20; i++ {
		if _, err := service.AddAffinity(ctx, db, aID, bID, 10); err != nil {
			t.Fatal(err)
		}
	}
	rel, err := service.GetOrCreateRelationship(ctx, db, aID, bID)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Affinity != 100 {
		t.Fatalf("expected affinity capped at 100, got %d", rel.Affinity)
	}
	if rel.Level != "best_friend" {
		t.Fatalf("expected best_friend, got %s", rel.Level)
	}
}
