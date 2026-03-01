// Package router wires all HTTP routes to their handler implementations.
package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/config"
	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/internal/gameconfig"
	"github.com/liukai/farmer/server/internal/handler"
	"github.com/liukai/farmer/server/internal/llm"
	"github.com/liukai/farmer/server/internal/middleware"
	"github.com/liukai/farmer/server/internal/ws"
)

// New builds and returns a fully-configured *gin.Engine.
//
// Route layout:
//
//	/api/v1/auth            — public (no JWT required)
//	/api/v1/payment/...     — mixed: callback is public, create-order is authed
//	everything else         — JWT-protected
func New(cfg *config.Config, db *ent.Client, hub *ws.Hub, llmSvc *llm.Service, gameCfg *gameconfig.Config) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// ── Global middleware ────────────────────────────────────────────────────
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.CORS(cfg.Server.Mode))
	r.Use(middleware.SecurityHeaders())

	// Global rate limiter: 300 req / min per IP (adjust as needed).
	globalRL := middleware.NewRateLimiter(300, time.Minute)
	r.Use(globalRL.Middleware())

	// Reject request bodies larger than 1 MB to prevent payload flooding.
	r.Use(middleware.MaxBodySize(1 << 20))

	// ── Static sprites (character PNGs for WorldMap rendering) ───────────────
	if cfg.Server.SpritesDir != "" {
		r.Static("/sprites", cfg.Server.SpritesDir)
	}

	// ── Health / readiness (T-095: disaster recovery probes) ─────────────────
	healthH := handler.NewHealthHandler(db)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/healthz", healthH.Liveness)  // liveness: process alive
	r.GET("/readyz", healthH.Readiness)  // readiness: DB reachable

	// ── Instantiate handlers ─────────────────────────────────────────────────
	authH := handler.NewAuthHandler(db, cfg.JWT.Secret, cfg.JWT.ExpiresHours, gameCfg)
	userH := handler.NewUserHandler(db)
	farmH := handler.NewFarmHandler(db)
	buildH := handler.NewBuildingHandler()
	animalH := handler.NewAnimalHandler(db)
	invH := handler.NewInventoryHandler(db)
	roleH := handler.NewRoleHandler(db, llmSvc, gameCfg)
	villageH := handler.NewVillageHandler(db)
	tradeH := handler.NewTradeHandler(db, hub)
	friendH := handler.NewFriendHandler(db)
	socialH := handler.NewSocialHandler(db)
	notifH := handler.NewNotificationHandler(db, hub)
	dailyH := handler.NewDailyHandler(db)
	shopH := handler.NewShopHandler(db)
	seasonH := handler.NewSeasonHandler(db)
	payH := handler.NewPaymentHandler(db)
	wsH := handler.NewWSHandler(hub, db, cfg.JWT.Secret)
	activityH := handler.NewActivityHandler(db)
	tutorialH := handler.NewTutorialHandler(db)
	chatH := handler.NewChatHandler(db)
	delegateH := handler.NewDelegateHandler(db)

	// ── Per-route rate limiters (stricter than global) ───────────────────────
	// Auth: 10 req/min — brute-force prevention on login/refresh endpoints.
	authRL := middleware.NewRateLimiter(10, time.Minute)
	// Daily check-in: 5 req/min — prevent reward spam.
	checkinRL := middleware.NewRateLimiter(5, time.Minute)
	// Trade buy: 30 req/min — prevent automated purchase bots.
	tradeBuyRL := middleware.NewRateLimiter(30, time.Minute)

	// ── API v1 ───────────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")

	// -- WebSocket (public endpoint, self-authenticates via ?token=) ----------
	v1.GET("/ws", wsH.Connect)

	// -- Auth (public) --------------------------------------------------------
	authGroup := v1.Group("/auth")
	authGroup.Use(authRL.Middleware()) // 10 req/min per IP: brute-force guard
	{
		authGroup.POST("/register", authH.Register)
		authGroup.POST("/login", authH.Login)
		authGroup.POST("/wx-login", authH.WxLogin)
		authGroup.POST("/refresh", authH.Refresh)
		authGroup.POST("/dev-login", authH.DevLogin) // H5 dev only
	}

	// -- Payment: callback is public, create-order requires auth --------------
	payGroup := v1.Group("/payment")
	{
		// WeChat Pay gateway calls this without any Bearer token.
		payGroup.POST("/wx-callback", payH.WxCallback)

		// Everything else in /payment needs authentication.
		payAuth := payGroup.Group("", middleware.Auth(cfg.JWT.Secret))
		payAuth.GET("/packages", payH.ListPackages)
		payAuth.POST("/orders", payH.CreateOrder)
	}

	// -- All remaining routes require a valid JWT ----------------------------
	authed := v1.Group("", middleware.Auth(cfg.JWT.Secret))

	// /users
	users := authed.Group("/users")
	{
		users.GET("/me", userH.GetMe)
		users.PUT("/me", userH.UpdateMe)
		users.GET("/:id", userH.GetByID)
	}

	// /farms
	farms := authed.Group("/farms")
	{
		farms.GET("/mine", farmH.GetMine)
		// T-081: farm delegation report (must appear before /:id routes)
		farms.GET("/delegate/report", delegateH.GetDelegateReport)
		farms.GET("/:id", farmH.GetByID)
		farms.POST("/:id/plant", farmH.Plant)
		farms.POST("/:id/water", farmH.Water)
		farms.POST("/:id/harvest", farmH.Harvest)
		farms.POST("/:id/visit", farmH.Visit)
		// T-081: delegate a friend's farm tending
		farms.POST("/delegate", delegateH.DelegateFarm)
	}

	// /buildings
	buildings := authed.Group("/buildings")
	{
		buildings.GET("", buildH.List)
		buildings.POST("", buildH.Create)
		buildings.PUT("/:id/level", buildH.UpgradeLevel)
		buildings.POST("/:id/finish-early", buildH.FinishEarly)
	}

	// /animals
	animals := authed.Group("/animals")
	{
		animals.GET("", animalH.List)
		// T-088: species catalog with unlock status (must appear before /:id routes)
		animals.GET("/catalog", animalH.Catalog)
		animals.POST("", animalH.Create)
		animals.POST("/:id/feed", animalH.Feed)
		animals.POST("/:id/clean", animalH.Clean)
		animals.POST("/:id/collect", animalH.Collect)
		animals.POST("/:id/pet", animalH.Pet)
	}

	// /inventory
	inventory := authed.Group("/inventory")
	{
		inventory.GET("", invH.GetInventory)
		inventory.POST("/sell", invH.SellItem)
	}

	// /agent
	roleGroup := authed.Group("/agent")
	{
		roleGroup.GET("", roleH.GetAgent)
		roleGroup.PUT("", roleH.UpdateAgent)
		roleGroup.POST("/chat", roleH.Chat)
		roleGroup.GET("/strategy", roleH.GetStrategy)
	}


	// /villages
	villages := authed.Group("/villages")
	{
		villages.GET("", villageH.List)
		villages.GET("/mine", villageH.Mine)
		villages.POST("", villageH.Create)
		// T-065: caller's village project list (no village ID in path — uses JWT identity)
		villages.GET("/projects", villageH.MyProjects)
		// T-065: contribute resources to a project by project ID
		villages.POST("/projects/:id/contribute", villageH.ContributeResource)
		// T-080: cross-village social exploration
		villages.GET("/explore", villageH.Explore)
		villages.GET("/:id", villageH.GetByID)
		villages.POST("/:id/join", villageH.Join)
		villages.POST("/:id/leave", villageH.Leave)
		villages.GET("/:id/projects", villageH.ListProjects)
		villages.POST("/:id/projects", villageH.CreateProject)
		villages.POST("/:id/projects/:projectID/contribute", villageH.ContributeProject)
		// T-080: list village members for cross-village friend-making
		villages.GET("/:id/members", villageH.ListMembers)
	}

	// /trade
	trade := authed.Group("/trade")
	{
		trade.GET("/orders", tradeH.ListOrders)
		trade.POST("/orders", tradeH.CreateOrder)
		trade.POST("/orders/:id/buy", tradeBuyRL.Middleware(), tradeH.BuyOrder) // 30 req/min: bot prevention
		// T-064: NPC merchant daily rotation (no DB required)
		trade.GET("/npc-listings", tradeH.NPCListings)
	}

	// /friends
	friends := authed.Group("/friends")
	{
		friends.GET("", friendH.ListFriends)
		friends.GET("/requests", friendH.ListRequests)
		friends.POST("/requests", friendH.AddFriend)
		friends.POST("/requests/:id/accept", friendH.AcceptRequest)
		friends.POST("/requests/:id/reject", friendH.RejectRequest)
	}

	// /social
	social := authed.Group("/social")
	{
		social.GET("/relationships", socialH.GetRelationships)
		social.GET("/feed", socialH.GetFeed)
		social.POST("/activities/:id/like", socialH.LikeActivity)
		social.POST("/activities/:id/comment", socialH.CommentActivity)
		// T-060: Gift system
		social.POST("/gift", socialH.SendGift)
		social.GET("/gifts", socialH.ListGifts)
		// T-061: Help-request system
		social.POST("/help-request", socialH.CreateHelpRequest)
		social.POST("/help-respond", socialH.RespondHelpRequest)
		social.GET("/help-requests", socialH.ListHelpRequests)
	}

	// /notifications
	notifications := authed.Group("/notifications")
	{
		notifications.GET("", notifH.List)
		notifications.PUT("/read-all", notifH.MarkAllRead)
		notifications.PUT("/:id/read", notifH.MarkRead)
	}

	// /daily
	daily := authed.Group("/daily")
	{
		daily.POST("/checkin", checkinRL.Middleware(), dailyH.CheckIn) // 5 req/min: reward spam guard
		daily.GET("/streak", dailyH.GetStreak)
	}

	// /shop
	shop := authed.Group("/shop")
	{
		shop.GET("/items", shopH.ListItems)
		shop.POST("/items/:id/buy", shopH.BuyItem)
		shop.GET("/friendship", shopH.ListFriendshipShop)
		shop.POST("/friendship/exchange", shopH.ExchangeFriendship)
	}

	// /seasons
	seasons := authed.Group("/seasons")
	{
		seasons.GET("/current", seasonH.GetCurrent)
		seasons.GET("/current/leaderboard", seasonH.GetLeaderboard)
		seasons.GET("/current/tasks", seasonH.GetTasks)
		seasons.GET("/current/village-leaderboard", seasonH.GetVillageLeaderboard)
	}

	// /activity
	activity := authed.Group("/activity")
	{
		activity.GET("/offline-summary", activityH.OfflineSummary)
	}

	// /tutorial
	tutorial := authed.Group("/tutorial")
	{
		tutorial.GET("/progress", tutorialH.GetProgress)
		tutorial.POST("/complete-step", tutorialH.CompleteStep)
	}

	// /chat
	chat := authed.Group("/chat")
	{
		chat.GET("/history", chatH.GetHistory)
		chat.POST("/send", chatH.Send)
	}

	// /achievements — T-092: achievement system
	achieveH := handler.NewAchievementHandler(db)
	authed.GET("/achievements", achieveH.ListAchievements)

	// /admin — T-075: operational metrics dashboard (JWT-authed; restrict to admin role in production)
	adminH := handler.NewAdminHandler(db)
	admin := authed.Group("/admin")
	{
		admin.GET("/metrics", adminH.Metrics)
		// T-093: admin management endpoints
		admin.GET("/users", adminH.ListUsers)
		admin.GET("/villages", adminH.ListVillages)
	}

	// /events — T-085: festival activity framework (calendar-based, no DB table needed)
	eventH := handler.NewEventActivityHandler(db)
	authed.GET("/events", eventH.ListEvents)
	authed.GET("/events/:type/status", eventH.GetEventStatus)

	// /share — T-086: WeChat fission sharing & invite rewards
	shareH := handler.NewShareHandler(db)
	authed.POST("/share/record", shareH.RecordShare)
	authed.GET("/share/invite-code", shareH.GetInviteCode)
	authed.POST("/share/invite-reward", shareH.ClaimInviteReward)

	// /subscriptions — T-087: WeChat message subscription preferences
	subH := handler.NewSubscriptionHandler(db, hub)
	subscriptions := authed.Group("/subscriptions")
	{
		subscriptions.GET("", subH.GetSubscriptions)
		subscriptions.POST("", subH.UpdateSubscription)
		// Internal: push a subscription message; restrict to internal callers in production.
		subscriptions.POST("/send", subH.SendSubscriptionMessage)
	}

	// /workshop — T-089: Workshop / processing system
	workshopH := handler.NewWorkshopHandler(db)
	workshop := authed.Group("/workshop")
	{
		workshop.GET("/recipes", workshopH.ListRecipes)
		workshop.POST("/start", workshopH.StartProcessing)
		workshop.POST("/collect", workshopH.CollectProduct)
		workshop.GET("/jobs", workshopH.ListJobs)
	}

	// /monthly-card — T-090: Monthly subscription card
	mcH := handler.NewMonthlyCardHandler(db)
	monthlyCard := authed.Group("/monthly-card")
	{
		monthlyCard.GET("/status", mcH.GetStatus)
		monthlyCard.POST("/purchase", mcH.PurchaseCard)
		monthlyCard.POST("/daily-reward", mcH.DailyReward)
		monthlyCard.GET("/perks", mcH.ListPerks)
	}

	// /collection — T-091: Collection encyclopedia
	collH := handler.NewCollectionHandler(db)
	collection := authed.Group("/collection")
	{
		collection.GET("", collH.ListCollection)
		collection.GET("/progress", collH.GetProgress)
	}

	return r
}
