// Package service contains domain business logic.
package service

import "time"

// CropDef describes a single crop type.
type CropDef struct {
	ID           string
	Name         string
	GrowDuration time.Duration // time from planting to harvestable
	YieldMin     int           // min items produced on harvest
	YieldMax     int           // max items produced on harvest
	CoinReward   int64         // coins awarded on harvest
	ExpReward    int           // exp awarded on harvest
	StaminaCost  int           // stamina to plant manually
	UnlockLevel  int           // player level required
}

// Crops is the static crop catalogue.
// IDs match the crop sprite frames used by the frontend.
var Crops = map[string]*CropDef{
	"turnip": {
		ID: "turnip", Name: "萝卜",
		GrowDuration: 2 * time.Minute,
		YieldMin: 1, YieldMax: 2, CoinReward: 20, ExpReward: 5,
		StaminaCost: 2, UnlockLevel: 1,
	},
	"potato": {
		ID: "potato", Name: "土豆",
		GrowDuration: 4 * time.Minute,
		YieldMin: 2, YieldMax: 3, CoinReward: 35, ExpReward: 8,
		StaminaCost: 2, UnlockLevel: 1,
	},
	"wheat": {
		ID: "wheat", Name: "小麦",
		GrowDuration: 6 * time.Minute,
		YieldMin: 2, YieldMax: 4, CoinReward: 50, ExpReward: 12,
		StaminaCost: 2, UnlockLevel: 2,
	},
	"carrot": {
		ID: "carrot", Name: "胡萝卜",
		GrowDuration: 8 * time.Minute,
		YieldMin: 2, YieldMax: 4, CoinReward: 60, ExpReward: 15,
		StaminaCost: 3, UnlockLevel: 3,
	},
	"tomato": {
		ID: "tomato", Name: "番茄",
		GrowDuration: 12 * time.Minute,
		YieldMin: 3, YieldMax: 5, CoinReward: 80, ExpReward: 20,
		StaminaCost: 3, UnlockLevel: 4,
	},
	"corn": {
		ID: "corn", Name: "玉米",
		GrowDuration: 20 * time.Minute,
		YieldMin: 3, YieldMax: 6, CoinReward: 120, ExpReward: 30,
		StaminaCost: 4, UnlockLevel: 5,
	},
	"strawberry": {
		ID: "strawberry", Name: "草莓",
		GrowDuration: 30 * time.Minute,
		YieldMin: 4, YieldMax: 8, CoinReward: 200, ExpReward: 50,
		StaminaCost: 5, UnlockLevel: 7,
	},
	"pumpkin": {
		ID: "pumpkin", Name: "南瓜",
		GrowDuration: 60 * time.Minute,
		YieldMin: 5, YieldMax: 10, CoinReward: 350, ExpReward: 80,
		StaminaCost: 6, UnlockLevel: 10,
	},
}

// GetCrop returns a CropDef by ID, or nil if unknown.
func GetCrop(id string) *CropDef { return Crops[id] }

// AllCrops returns all crop definitions as a slice.
func AllCrops() []*CropDef {
	result := make([]*CropDef, 0, len(Crops))
	for _, c := range Crops {
		result = append(result, c)
	}
	return result
}
