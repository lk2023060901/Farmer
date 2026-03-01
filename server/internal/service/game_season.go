// Package service contains domain business logic.
package service

import "time"

// GameSeason represents the current in-game season.
type GameSeason string

const (
	SeasonSpring GameSeason = "spring" // week % 4 == 0
	SeasonSummer GameSeason = "summer" // week % 4 == 1
	SeasonFall   GameSeason = "fall"   // week % 4 == 2
	SeasonWinter GameSeason = "winter" // week % 4 == 3
)

// seasonBonusCrops maps each season to crop IDs that receive a 50% output bonus.
var seasonBonusCrops = map[GameSeason][]string{
	SeasonSpring: {"turnip", "carrot"},
	SeasonSummer: {"tomato", "corn"},
	SeasonFall:   {"potato", "pumpkin"},
	SeasonWinter: {"wheat"},
}

// seasonForbiddenCrops maps each season to crop IDs that cannot be planted.
var seasonForbiddenCrops = map[GameSeason][]string{
	SeasonWinter: {"strawberry", "tomato", "corn"},
}

// CurrentGameSeason returns the current in-game season derived from the real-world ISO week number.
// 1 real week = 1 game season; cycles spring → summer → fall → winter.
func CurrentGameSeason() GameSeason {
	_, week := time.Now().ISOWeek()
	switch week % 4 {
	case 0:
		return SeasonSpring
	case 1:
		return SeasonSummer
	case 2:
		return SeasonFall
	default:
		return SeasonWinter
	}
}

// GetSeasonCoinBonus returns 1.5 if cropID is a bonus crop this season, else 1.0.
func GetSeasonCoinBonus(cropID string) float64 {
	for _, id := range seasonBonusCrops[CurrentGameSeason()] {
		if id == cropID {
			return 1.5
		}
	}
	return 1.0
}

// IsCropForbidden returns true if cropID cannot be planted in the current season.
func IsCropForbidden(cropID string) bool {
	for _, id := range seasonForbiddenCrops[CurrentGameSeason()] {
		if id == cropID {
			return true
		}
	}
	return false
}

// CurrentSeasonInfo returns a map of season metadata for API responses.
func CurrentSeasonInfo() map[string]interface{} {
	season := CurrentGameSeason()
	bonus := seasonBonusCrops[season]
	forbidden := seasonForbiddenCrops[season]
	if forbidden == nil {
		forbidden = []string{}
	}
	weekday := int(time.Now().Weekday())
	daysLeft := 7 - weekday
	if daysLeft == 7 {
		daysLeft = 0
	}
	return map[string]interface{}{
		"season":         string(season),
		"bonusCrops":     bonus,
		"forbiddenCrops": forbidden,
		"daysRemaining":  daysLeft,
	}
}
