package handler

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entanimal "github.com/liukai/farmer/server/ent/animal"
	entfarm "github.com/liukai/farmer/server/ent/farm"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	"github.com/liukai/farmer/server/internal/service"
)

// AnimalHandler groups animal-management route handlers.
type AnimalHandler struct {
	db *ent.Client
}

// NewAnimalHandler constructs an AnimalHandler.
func NewAnimalHandler(db *ent.Client) *AnimalHandler { return &AnimalHandler{db: db} }

// animalSpec defines production properties for each animal type.
type animalSpec struct {
	productItem string        // item_id added to inventory on collect
	cycleDur    time.Duration // production cycle duration
	baseQty     int           // base quantity produced per cycle
	buyCost     int           // purchase price in coins
}

var animalSpecs = map[string]animalSpec{
	"chicken": {productItem: "egg", cycleDur: 2 * time.Hour, baseQty: 3, buyCost: 200},
	"cow":     {productItem: "milk", cycleDur: 4 * time.Hour, baseQty: 2, buyCost: 500},
	"sheep":   {productItem: "wool", cycleDur: 6 * time.Hour, baseQty: 2, buyCost: 600},
	"bee":     {productItem: "honey", cycleDur: 8 * time.Hour, baseQty: 1, buyCost: 800},
	"rabbit":  {productItem: "rabbit_fur", cycleDur: 4 * time.Hour, baseQty: 2, buyCost: 400},
}

type animalDTO struct {
	ID             uuid.UUID  `json:"id"`
	FarmID         uuid.UUID  `json:"farmId"`
	Type           string     `json:"type"`
	Name           *string    `json:"name"`
	Mood           int        `json:"mood"`
	Health         int        `json:"health"`
	LastFedAt      *time.Time `json:"lastFedAt"`
	LastCleanedAt  *time.Time `json:"lastCleanedAt"`
	LastProductAt  *time.Time `json:"lastProductAt"`
	ReadyToCollect bool       `json:"readyToCollect"`
	ProductItem    string     `json:"productItem"`
}

func toAnimalDTO(a *ent.Animal) animalDTO {
	spec := animalSpecs[a.Type]
	ready := a.LastProductAt == nil || time.Since(*a.LastProductAt) >= spec.cycleDur
	return animalDTO{
		ID:             a.ID,
		FarmID:         a.FarmID,
		Type:           a.Type,
		Name:           a.Name,
		Mood:           a.Mood,
		Health:         a.Health,
		LastFedAt:      a.LastFedAt,
		LastCleanedAt:  a.LastCleanedAt,
		LastProductAt:  a.LastProductAt,
		ReadyToCollect: ready,
		ProductItem:    spec.productItem,
	}
}

// computeAnimalYield applies mood-based multiplier:
//   - mood ≥ 80 → 1.5× base + 20% chance of +1 bonus
//   - mood ≤ 30 → 0.5× base (min 1)
//   - otherwise → base qty
func computeAnimalYield(base, mood int) int {
	switch {
	case mood >= 80:
		qty := int(float64(base) * 1.5)
		if rand.Intn(100) < 20 {
			qty++
		}
		return qty
	case mood <= 30:
		qty := base / 2
		if qty < 1 {
			return 1
		}
		return qty
	default:
		return base
	}
}

// getOwnedAnimal parses :id, verifies the animal belongs to the authenticated user's farm.
func (h *AnimalHandler) getOwnedAnimal(c *gin.Context) (*ent.Animal, *ent.Farm, bool) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return nil, nil, false
	}
	animalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return nil, nil, false
	}
	ctx := c.Request.Context()
	a, err := h.db.Animal.Get(ctx, animalID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "animal not found", "data": nil})
		return nil, nil, false
	}
	farm, err := h.db.Farm.Query().Where(entfarm.OwnerID(userID)).Only(ctx)
	if err != nil || farm.ID != a.FarmID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden", "data": nil})
		return nil, nil, false
	}
	return a, farm, true
}

// List handles GET /api/v1/animals
// Returns all animals on the authenticated user's farm.
func (h *AnimalHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()
	farm, err := h.db.Farm.Query().Where(entfarm.OwnerID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}
	animals, err := h.db.Animal.Query().
		Where(entanimal.FarmID(farm.ID)).
		Order(ent.Asc(entanimal.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	result := make([]animalDTO, len(animals))
	for i, a := range animals {
		result[i] = toAnimalDTO(a)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

type createAnimalReq struct {
	Type string  `json:"type" binding:"required"`
	Name *string `json:"name"`
}

// Create handles POST /api/v1/animals
// Purchases a new animal for the player's farm, deducting coins.
func (h *AnimalHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req createAnimalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}
	spec, exists := animalSpecs[req.Type]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "unknown animal type", "data": nil})
		return
	}
	ctx := c.Request.Context()
	farm, err := h.db.Farm.Query().Where(entfarm.OwnerID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}
	u, err := h.db.User.Get(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}
	if u.Coins < int64(spec.buyCost) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "insufficient coins", "data": nil})
		return
	}
	if err := h.db.User.UpdateOneID(userID).AddCoins(-int64(spec.buyCost)).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "deduct coins failed", "data": nil})
		return
	}
	mut := h.db.Animal.Create().SetFarmID(farm.ID).SetType(req.Type)
	if req.Name != nil {
		mut = mut.SetName(*req.Name)
	}
	a, err := mut.Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": toAnimalDTO(a)})
}

// Feed handles POST /api/v1/animals/:id/feed — mood +10, records last_fed_at.
func (h *AnimalHandler) Feed(c *gin.Context) {
	a, _, ok := h.getOwnedAnimal(c)
	if !ok {
		return
	}
	newMood := a.Mood + 10
	if newMood > 100 {
		newMood = 100
	}
	updated, err := h.db.Animal.UpdateOneID(a.ID).
		SetMood(newMood).
		SetLastFedAt(time.Now()).
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": toAnimalDTO(updated)})
}

// Clean handles POST /api/v1/animals/:id/clean — mood +5, records last_cleaned_at.
func (h *AnimalHandler) Clean(c *gin.Context) {
	a, _, ok := h.getOwnedAnimal(c)
	if !ok {
		return
	}
	newMood := a.Mood + 5
	if newMood > 100 {
		newMood = 100
	}
	updated, err := h.db.Animal.UpdateOneID(a.ID).
		SetMood(newMood).
		SetLastCleanedAt(time.Now()).
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": toAnimalDTO(updated)})
}

// Pet handles POST /api/v1/animals/:id/pet — mood +15.
func (h *AnimalHandler) Pet(c *gin.Context) {
	a, _, ok := h.getOwnedAnimal(c)
	if !ok {
		return
	}
	newMood := a.Mood + 15
	if newMood > 100 {
		newMood = 100
	}
	updated, err := h.db.Animal.UpdateOneID(a.ID).
		SetMood(newMood).
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": toAnimalDTO(updated)})
}

// Collect handles POST /api/v1/animals/:id/collect
// Checks cycle completion, computes yield with mood multiplier, adds to inventory.
func (h *AnimalHandler) Collect(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	animalID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}
	ctx := c.Request.Context()
	a, err := h.db.Animal.Get(ctx, animalID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "animal not found", "data": nil})
		return
	}
	farm, err := h.db.Farm.Query().Where(entfarm.OwnerID(userID)).Only(ctx)
	if err != nil || farm.ID != a.FarmID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden", "data": nil})
		return
	}

	spec := animalSpecs[a.Type]
	now := time.Now()

	// Check if cycle is complete.
	if a.LastProductAt != nil && time.Since(*a.LastProductAt) < spec.cycleDur {
		remaining := spec.cycleDur - time.Since(*a.LastProductAt)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "not ready yet",
			"data":    gin.H{"remainingSecs": int(remaining.Seconds())},
		})
		return
	}

	// Unhappy animals (mood ≤ 30) have a 30% chance of missing production.
	if a.Mood <= 30 && rand.Intn(100) < 30 {
		_ = h.db.Animal.UpdateOneID(a.ID).SetLastProductAt(now).Exec(ctx)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "animal too unhappy to produce", "data": gin.H{"quantity": 0}})
		return
	}

	qty := computeAnimalYield(spec.baseQty, a.Mood)

	// Upsert inventory item.
	existing, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(spec.productItem), entinv.ItemType("product")).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = h.db.InventoryItem.Create().
			SetUserID(userID).
			SetItemID(spec.productItem).
			SetItemType("product").
			SetQuantity(int64(qty)).
			Save(ctx)
	} else if err == nil {
		err = h.db.InventoryItem.UpdateOneID(existing.ID).AddQuantity(int64(qty)).Exec(ctx)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	_ = h.db.Animal.UpdateOneID(a.ID).SetLastProductAt(now).Exec(ctx)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"itemId": spec.productItem, "quantity": qty},
	})
}

// Catalog handles GET /api/v1/animals/catalog
// Returns the full static species catalogue (T-088), including unlock levels and
// costs, so the frontend can render the "Animal Shop" with locked/unlocked states
// based on the caller's current player level.
func (h *AnimalHandler) Catalog(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()
	u, err := h.db.User.Get(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}

	type catalogEntry struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		Emoji           string  `json:"emoji"`
		ProductID       string  `json:"productId"`
		ProductName     string  `json:"productName"`
		FeedIntervalH   float64 `json:"feedIntervalH"`
		ProductionCycleH float64 `json:"productionCycleH"`
		BaseProductQty  int     `json:"baseProductQty"`
		BuyCost         int     `json:"buyCost"`
		UnlockLevel     int     `json:"unlockLevel"`
		Unlocked        bool    `json:"unlocked"`
	}

	defs := service.AllAnimals()
	entries := make([]catalogEntry, 0, len(defs))
	for _, d := range defs {
		entries = append(entries, catalogEntry{
			ID:              d.ID,
			Name:            d.Name,
			Emoji:           d.Emoji,
			ProductID:       d.ProductID,
			ProductName:     d.ProductName,
			FeedIntervalH:   d.FeedInterval.Hours(),
			ProductionCycleH: d.ProductionCycle.Hours(),
			BaseProductQty:  d.BaseProductQty,
			BuyCost:         d.BuyCost,
			UnlockLevel:     d.UnlockLevel,
			Unlocked:        u.Level >= d.UnlockLevel,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": entries})
}
