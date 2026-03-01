package handler

import (
	"testing"
	"time"
)

// TestTodayUTC verifies todayUTC() returns midnight UTC for today.
func TestTodayUTC(t *testing.T) {
	result := todayUTC()

	// Must be at 00:00:00.000000000 UTC
	if result.Hour() != 0 || result.Minute() != 0 || result.Second() != 0 || result.Nanosecond() != 0 {
		t.Errorf("todayUTC() time-of-day = %02d:%02d:%02d.%09d, want 00:00:00.000000000",
			result.Hour(), result.Minute(), result.Second(), result.Nanosecond())
	}

	if result.Location() != time.UTC {
		t.Errorf("todayUTC() location = %v, want UTC", result.Location())
	}

	// Must match today's date in UTC
	now := time.Now().UTC()
	if result.Year() != now.Year() || result.Month() != now.Month() || result.Day() != now.Day() {
		t.Errorf("todayUTC() date = %v, want %04d-%02d-%02d",
			result.Format("2006-01-02"), now.Year(), now.Month(), now.Day())
	}
}

// TestCheckinRewardsLength verifies the 7-day reward array has exactly 7 entries.
func TestCheckinRewardsLength(t *testing.T) {
	if len(checkinRewards) != 7 {
		t.Errorf("checkinRewards length = %d, want 7", len(checkinRewards))
	}
}

// TestCheckinRewardsDay7Diamonds verifies index 6 (Day 7) is diamonds.
func TestCheckinRewardsDay7Diamonds(t *testing.T) {
	day7 := checkinRewards[6]
	if day7.Type != "diamonds" {
		t.Errorf("checkinRewards[6].Type = %q, want %q", day7.Type, "diamonds")
	}
	if day7.Quantity <= 0 {
		t.Errorf("checkinRewards[6].Quantity = %d, want > 0", day7.Quantity)
	}
}

// TestCheckinRewardsDay4Stamina verifies index 3 (Day 4) is stamina.
func TestCheckinRewardsDay4Stamina(t *testing.T) {
	day4 := checkinRewards[3]
	if day4.Type != "stamina" {
		t.Errorf("checkinRewards[3].Type = %q, want %q", day4.Type, "stamina")
	}
	if day4.Quantity <= 0 {
		t.Errorf("checkinRewards[3].Quantity = %d, want > 0", day4.Quantity)
	}
}

// TestCheckinRewardsAllValid verifies every entry in checkinRewards has a valid type and positive quantity.
func TestCheckinRewardsAllValid(t *testing.T) {
	validTypes := map[string]bool{"coins": true, "diamonds": true, "stamina": true}
	for i, r := range checkinRewards {
		if !validTypes[r.Type] {
			t.Errorf("checkinRewards[%d].Type = %q, not a valid reward type", i, r.Type)
		}
		if r.Quantity <= 0 {
			t.Errorf("checkinRewards[%d].Quantity = %d, want > 0", i, r.Quantity)
		}
	}
}

// TestCycleDayCalculation tests the core cycle-day math: cycleDay = (streak - 1) % 7
// This is the exact formula used in CheckIn handler.
func TestCycleDayCalculation(t *testing.T) {
	tests := []struct {
		name        string
		streak      int
		wantCycleDay int
	}{
		{
			name:         "streak=1 yields cycleDay=0 (Day 1 reward)",
			streak:       1,
			wantCycleDay: 0,
		},
		{
			name:         "streak=7 yields cycleDay=6 (Day 7 diamonds reward)",
			streak:       7,
			wantCycleDay: 6,
		},
		{
			name:         "streak=8 yields cycleDay=0 (cycle resets to Day 1)",
			streak:       8,
			wantCycleDay: 0,
		},
		{
			name:         "streak=14 yields cycleDay=6 (second cycle Day 7)",
			streak:       14,
			wantCycleDay: 6,
		},
		{
			name:         "streak=2 yields cycleDay=1 (Day 2 reward)",
			streak:       2,
			wantCycleDay: 1,
		},
		{
			name:         "streak=3 yields cycleDay=2 (Day 3 reward)",
			streak:       3,
			wantCycleDay: 2,
		},
		{
			name:         "streak=4 yields cycleDay=3 (Day 4 stamina reward)",
			streak:       4,
			wantCycleDay: 3,
		},
		{
			name:         "streak=21 yields cycleDay=6 (third cycle Day 7)",
			streak:       21,
			wantCycleDay: 6,
		},
		{
			name:         "streak=15 yields cycleDay=0 (third cycle Day 1)",
			streak:       15,
			wantCycleDay: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cycleDay := (tc.streak - 1) % 7
			if cycleDay != tc.wantCycleDay {
				t.Errorf("(streak=%d - 1) %% 7 = %d, want %d", tc.streak, cycleDay, tc.wantCycleDay)
			}
			// Also verify that the cycleDay correctly indexes into checkinRewards
			reward := checkinRewards[cycleDay]
			if reward.Type == "" {
				t.Errorf("checkinRewards[%d].Type is empty for streak=%d", cycleDay, tc.streak)
			}
		})
	}
}

// TestCycleDayDay7GivesDiamonds confirms that streak=7 and streak=14 both land on diamonds.
func TestCycleDayDay7GivesDiamonds(t *testing.T) {
	for _, streak := range []int{7, 14, 21} {
		cycleDay := (streak - 1) % 7
		reward := checkinRewards[cycleDay]
		if reward.Type != "diamonds" {
			t.Errorf("streak=%d -> cycleDay=%d -> reward.Type=%q, want %q",
				streak, cycleDay, reward.Type, "diamonds")
		}
	}
}

// TestCycleDayDay4GivesStamina confirms that streak=4 and streak=11 both land on stamina.
func TestCycleDayDay4GivesStamina(t *testing.T) {
	for _, streak := range []int{4, 11, 18} {
		cycleDay := (streak - 1) % 7
		reward := checkinRewards[cycleDay]
		if reward.Type != "stamina" {
			t.Errorf("streak=%d -> cycleDay=%d -> reward.Type=%q, want %q",
				streak, cycleDay, reward.Type, "stamina")
		}
	}
}
