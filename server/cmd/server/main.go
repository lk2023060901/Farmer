// Package main is the entry point for the 农趣村 (NongQuCun) game server.
//
//	@title			农趣村 API
//	@version		1.0
//	@description	Backend API for the NongQuCun multiplayer farming game.
//	@termsOfService	http://swagger.io/terms/
//
//	@contact.name	NongQuCun Team
//	@contact.email	dev@nongqucun.example.com
//
//	@license.name	Proprietary
//
//	@host		localhost:9080
//	@BasePath	/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer " followed by your JWT access token.
package main

import (
	"context"
	"log"

	"github.com/liukai/farmer/server/config"
	"github.com/liukai/farmer/server/internal/gameconfig"
	"github.com/liukai/farmer/server/internal/llm"
	"github.com/liukai/farmer/server/internal/router"
	"github.com/liukai/farmer/server/internal/store"
	"github.com/liukai/farmer/server/internal/tick"
	"github.com/liukai/farmer/server/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	gameCfg, err := gameconfig.Load()
	if err != nil {
		log.Fatalf("failed to load game config: %v", err)
	}
	log.Printf("game config loaded: %d items, default role_id=%d",
		len(gameCfg.Global.Items), gameCfg.Global.RoleID)

	db, err := store.Open(cfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	log.Println("database connected and migrated")

	// Connect Redis (non-fatal if unavailable).
	rdb, err := store.OpenRedis(cfg)
	if err != nil {
		log.Printf("redis connect error (non-fatal): %v", err)
	}
	if rdb != nil {
		defer rdb.Close()
		log.Println("redis connected")
	} else {
		log.Println("redis unavailable — LLM caching and quota tracking disabled")
	}

	// Build LLM service (reads API key from LLM_API_KEY env, falls back to templates if empty).
	llmCfg := llm.DefaultClientConfig()
	llmSvc := llm.NewService(llmCfg, rdb)

	// Start WebSocket hub
	hub := ws.NewHub()

	// Start background engines.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine := tick.New(db, hub)
	engine.Start(ctx)          // crop growth + agent actions (60s)
	engine.StartMovement(ctx)  // role autonomous movement (5s)

	r := router.New(cfg, db, hub, llmSvc, gameCfg)

	addr := ":" + cfg.Server.Port
	log.Printf("starting 农趣村 server on %s (mode=%s)", addr, cfg.Server.Mode)

	if err := r.Run(addr); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
