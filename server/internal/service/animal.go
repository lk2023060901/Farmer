package service

import (
	"sort"
	"time"
)

// AnimalDef defines the static properties of an animal species.
// Game systems (handler, tick, AI agent) should use this as the single source
// of truth for species configuration.
type AnimalDef struct {
	// ID is the canonical species identifier stored in the DB animal.type column.
	ID string
	// Name is the display name shown to the player (Chinese).
	Name string
	// ProductID is the item_id added to the player's inventory on collection.
	ProductID string
	// ProductName is the human-readable product label.
	ProductName string
	// FeedInterval is how often the animal needs feeding before mood decays.
	FeedInterval time.Duration
	// MaxHappiness is the maximum mood value (always 100 for all species).
	MaxHappiness int
	// UnlockLevel is the player level required to purchase this species.
	UnlockLevel int
	// BuyCost is the coin price to purchase one animal of this species.
	BuyCost int
	// ProductionCycle is the time between product collection windows.
	ProductionCycle time.Duration
	// BaseProductQty is the base number of product units per collection.
	BaseProductQty int
	// Emoji is the unicode emoji used in the UI for this species.
	Emoji string
}

// Animals is the complete static catalogue of all supported animal species.
// Keys are the species ID strings that match the DB animal.type field and the
// frontend AnimalType union type.
var Animals = map[string]*AnimalDef{
	"chicken": {
		ID:              "chicken",
		Name:            "母鸡",
		ProductID:       "egg",
		ProductName:     "鸡蛋",
		FeedInterval:    4 * time.Hour,
		MaxHappiness:    100,
		UnlockLevel:     1,
		BuyCost:         200,
		ProductionCycle: 2 * time.Hour,
		BaseProductQty:  3,
		Emoji:           "🐔",
	},
	"cow": {
		ID:              "cow",
		Name:            "奶牛",
		ProductID:       "milk",
		ProductName:     "牛奶",
		FeedInterval:    6 * time.Hour,
		MaxHappiness:    100,
		UnlockLevel:     3,
		BuyCost:         500,
		ProductionCycle: 4 * time.Hour,
		BaseProductQty:  2,
		Emoji:           "🐄",
	},
	"bee": {
		ID:              "bee",
		Name:            "蜜蜂",
		ProductID:       "honey",
		ProductName:     "蜂蜜",
		FeedInterval:    8 * time.Hour,
		MaxHappiness:    100,
		UnlockLevel:     5,
		BuyCost:         800,
		ProductionCycle: 8 * time.Hour,
		BaseProductQty:  1,
		Emoji:           "🐝",
	},
	"rabbit": {
		ID:              "rabbit",
		Name:            "兔子",
		ProductID:       "rabbit_fur",
		ProductName:     "兔毛",
		FeedInterval:    4 * time.Hour,
		MaxHappiness:    100,
		UnlockLevel:     4,
		BuyCost:         400,
		ProductionCycle: 4 * time.Hour,
		BaseProductQty:  2,
		Emoji:           "🐰",
	},
	"sheep": {
		ID:              "sheep",
		Name:            "绵羊",
		ProductID:       "wool",
		ProductName:     "羊毛",
		FeedInterval:    6 * time.Hour,
		MaxHappiness:    100,
		UnlockLevel:     6,
		BuyCost:         600,
		ProductionCycle: 6 * time.Hour,
		BaseProductQty:  2,
		Emoji:           "🐑",
	},
}

// GetAnimal returns the AnimalDef for the given species ID, or nil if unknown.
func GetAnimal(id string) *AnimalDef {
	return Animals[id]
}

// AllAnimals returns all AnimalDef entries in deterministic order (by UnlockLevel,
// then alphabetical ID) so the UI catalog always renders consistently.
func AllAnimals() []*AnimalDef {
	defs := make([]*AnimalDef, 0, len(Animals))
	for _, d := range Animals {
		defs = append(defs, d)
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].UnlockLevel != defs[j].UnlockLevel {
			return defs[i].UnlockLevel < defs[j].UnlockLevel
		}
		return defs[i].ID < defs[j].ID
	})
	return defs
}
