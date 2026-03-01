// Command migrate runs Ent's automatic schema migration against the database.
// Usage: go run ./cmd/migrate
// Reads DATABASE_DSN from environment (or .env file loaded externally).
package main

import (
	"context"
	"log"
	"os"

	"entgo.io/ent/dialect"
	"github.com/liukai/farmer/server/ent"
	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://farmer:farmer_secret@localhost:5432/farmer_dev?sslmode=disable"
	}

	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("failed to run schema migration: %v", err)
	}

	log.Println("schema migration completed successfully")
}
