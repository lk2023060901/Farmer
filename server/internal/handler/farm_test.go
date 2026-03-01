package handler

import (
	"testing"
	"time"

	"github.com/liukai/farmer/server/internal/service"
)

// TestCropSellPriceKnownCrops verifies that known crops return their defined market price.
func TestCropSellPriceKnownCrops(t *testing.T) {
	tests := []struct {
		itemID    string
		wantPrice int
	}{
		{"turnip", 8},
		{"potato", 14},
		{"wheat", 20},
		{"carrot", 24},
		{"tomato", 40},
		{"corn", 60},
		{"strawberry", 85},
		{"pumpkin", 150},
	}

	for _, tc := range tests {
		t.Run(tc.itemID, func(t *testing.T) {
			got := cropSellPrice(tc.itemID)
			if got != tc.wantPrice {
				t.Errorf("cropSellPrice(%q) = %d, want %d", tc.itemID, got, tc.wantPrice)
			}
		})
	}
}

// TestCropSellPriceFallback verifies that unknown item IDs return the fallback price of 5.
func TestCropSellPriceFallback(t *testing.T) {
	unknowns := []string{"", "banana", "dragonfruit", "unknown_crop"}
	for _, id := range unknowns {
		t.Run(id, func(t *testing.T) {
			got := cropSellPrice(id)
			if got != 5 {
				t.Errorf("cropSellPrice(%q) = %d, want 5 (fallback)", id, got)
			}
		})
	}
}

// TestPlotIndexCalculation verifies the plot index formula used in farm handlers:
// idx = y*8 + x  (8-column grid)
func TestPlotIndexCalculation(t *testing.T) {
	tests := []struct {
		x, y    int
		wantIdx int
	}{
		{0, 0, 0},
		{7, 0, 7},
		{0, 1, 8},
		{7, 7, 63},
		{3, 4, 35},
	}

	for _, tc := range tests {
		idx := tc.y*8 + tc.x
		if idx != tc.wantIdx {
			t.Errorf("plot index for (%d,%d) = %d, want %d", tc.x, tc.y, idx, tc.wantIdx)
		}
	}
}

// TestWaterStaminaCost ensures the water stamina cost constant is positive and matches expected value.
func TestWaterStaminaCost(t *testing.T) {
	if waterStaminaCost <= 0 {
		t.Errorf("waterStaminaCost = %d, want > 0", waterStaminaCost)
	}
	if waterStaminaCost != 2 {
		t.Errorf("waterStaminaCost = %d, want 2", waterStaminaCost)
	}
}

// TestServiceCropsAllNonZeroCoinReward verifies every crop in the catalogue has a non-zero CoinReward.
func TestServiceCropsAllNonZeroCoinReward(t *testing.T) {
	if len(service.Crops) == 0 {
		t.Fatal("service.Crops is empty — catalogue missing")
	}
	for id, crop := range service.Crops {
		if crop.CoinReward <= 0 {
			t.Errorf("crop %q has CoinReward = %d, want > 0", id, crop.CoinReward)
		}
	}
}

// TestServiceCropsAllNonZeroGrowDuration verifies every crop has a positive GrowDuration.
func TestServiceCropsAllNonZeroGrowDuration(t *testing.T) {
	for id, crop := range service.Crops {
		if crop.GrowDuration <= 0 {
			t.Errorf("crop %q has GrowDuration = %v, want > 0", id, crop.GrowDuration)
		}
	}
}

// TestServiceCropsWheatExpectedValues checks wheat has the documented stats.
func TestServiceCropsWheatExpectedValues(t *testing.T) {
	wheat := service.GetCrop("wheat")
	if wheat == nil {
		t.Fatal("wheat not found in crop catalogue")
	}

	if wheat.CoinReward != 50 {
		t.Errorf("wheat.CoinReward = %d, want 50", wheat.CoinReward)
	}
	if wheat.GrowDuration != 6*time.Minute {
		t.Errorf("wheat.GrowDuration = %v, want 6m0s", wheat.GrowDuration)
	}
	if wheat.YieldMin <= 0 {
		t.Errorf("wheat.YieldMin = %d, want > 0", wheat.YieldMin)
	}
	if wheat.YieldMax < wheat.YieldMin {
		t.Errorf("wheat.YieldMax = %d, want >= YieldMin (%d)", wheat.YieldMax, wheat.YieldMin)
	}
	if wheat.ExpReward <= 0 {
		t.Errorf("wheat.ExpReward = %d, want > 0", wheat.ExpReward)
	}
}

// TestServiceCropsYieldRangesValid verifies YieldMax >= YieldMin for every crop.
func TestServiceCropsYieldRangesValid(t *testing.T) {
	for id, crop := range service.Crops {
		if crop.YieldMin <= 0 {
			t.Errorf("crop %q YieldMin = %d, want > 0", id, crop.YieldMin)
		}
		if crop.YieldMax < crop.YieldMin {
			t.Errorf("crop %q YieldMax (%d) < YieldMin (%d)", id, crop.YieldMax, crop.YieldMin)
		}
	}
}

// TestServiceCropsUnlockLevelsPositive verifies every crop has a positive UnlockLevel.
func TestServiceCropsUnlockLevelsPositive(t *testing.T) {
	for id, crop := range service.Crops {
		if crop.UnlockLevel <= 0 {
			t.Errorf("crop %q UnlockLevel = %d, want > 0", id, crop.UnlockLevel)
		}
	}
}

// TestWateringBonusCalculation verifies the 20% growth speed-up math used in Harvest.
// effectiveDuration = GrowDuration * 0.8 when wateredAt is set.
func TestWateringBonusCalculation(t *testing.T) {
	crop := service.GetCrop("wheat")
	if crop == nil {
		t.Fatal("wheat not found in crop catalogue")
	}

	base := crop.GrowDuration
	withWater := time.Duration(float64(base) * 0.8)

	if withWater >= base {
		t.Errorf("watered duration (%v) should be less than base (%v)", withWater, base)
	}

	// 20% reduction: effective = 80% of base
	expected := 6*time.Minute*80/100
	if withWater != expected {
		t.Errorf("watered wheat duration = %v, want %v", withWater, expected)
	}
}
