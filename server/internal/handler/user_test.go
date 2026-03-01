package handler

import (
	"testing"
	"time"
)

func TestComputeStamina(t *testing.T) {
	// staminaRecoveryRate = 6*60 = 360 seconds per point

	tests := []struct {
		name        string
		stored      int
		max         int
		updatedAt   time.Time
		wantCurrent int
		wantChanged bool
	}{
		{
			name:        "stored equals max returns stored unchanged",
			stored:      100,
			max:         100,
			updatedAt:   time.Now().Add(-1 * time.Hour),
			wantCurrent: 100,
			wantChanged: false,
		},
		{
			name:        "elapsed less than one recovery rate returns stored unchanged",
			stored:      50,
			max:         100,
			updatedAt:   time.Now().Add(-5 * time.Minute), // 300s < 360s, recovers 0
			wantCurrent: 50,
			wantChanged: false,
		},
		{
			name:        "elapsed exactly zero returns stored unchanged",
			stored:      50,
			max:         100,
			updatedAt:   time.Now(),
			wantCurrent: 50,
			wantChanged: false,
		},
		{
			name:        "elapsed covers one recovery rate adds one point",
			stored:      50,
			max:         100,
			updatedAt:   time.Now().Add(-7 * time.Minute), // 420s / 360 = 1 recovered
			wantCurrent: 51,
			wantChanged: true,
		},
		{
			name:        "elapsed covers five recovery rates adds five points",
			stored:      40,
			max:         100,
			updatedAt:   time.Now().Add(-30 * time.Minute), // 1800s / 360 = 5 recovered
			wantCurrent: 45,
			wantChanged: true,
		},
		{
			name:        "computed stamina exceeds max is capped at max",
			stored:      90,
			max:         100,
			updatedAt:   time.Now().Add(-2 * time.Hour), // 7200s / 360 = 20; 90+20=110 > 100
			wantCurrent: 100,
			wantChanged: true,
		},
		{
			name:        "zero stored with lots of time recovered up to max",
			stored:      0,
			max:         100,
			updatedAt:   time.Now().Add(-12 * time.Hour), // 43200s / 360 = 120; capped at 100
			wantCurrent: 100,
			wantChanged: true,
		},
		{
			name:        "zero stored with exactly one recovery interval",
			stored:      0,
			max:         100,
			updatedAt:   time.Now().Add(-6 * time.Minute), // exactly 360s = 1 point
			wantCurrent: 1,
			wantChanged: true,
		},
		{
			name:        "stored at max minus one with insufficient time stays unchanged",
			stored:      99,
			max:         100,
			updatedAt:   time.Now().Add(-1 * time.Minute), // 60s < 360s, recovers 0
			wantCurrent: 99,
			wantChanged: false,
		},
		{
			name:        "stored at max with large elapsed still returns max unchanged",
			stored:      100,
			max:         100,
			updatedAt:   time.Now().Add(-24 * time.Hour),
			wantCurrent: 100,
			wantChanged: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCurrent, gotChanged := computeStamina(tc.stored, tc.max, tc.updatedAt)
			if gotCurrent != tc.wantCurrent {
				t.Errorf("current stamina = %d, want %d", gotCurrent, tc.wantCurrent)
			}
			if gotChanged != tc.wantChanged {
				t.Errorf("changed = %v, want %v", gotChanged, tc.wantChanged)
			}
		})
	}
}
