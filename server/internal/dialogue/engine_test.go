package dialogue_test

import (
	"strings"
	"testing"

	"github.com/liukai/farmer/server/internal/dialogue"
)

func TestGenerate_VisitReturnsLines(t *testing.T) {
	vars := dialogue.Vars{
		AgentName:  "小农",
		TargetName: "大壮",
		CropName:   "胡萝卜",
		Season:     "spring",
	}
	lines := dialogue.Generate("visit", "stranger", 5, vars)
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Text == "" {
			t.Fatal("empty line text")
		}
		if l.SpeakerRole != "caller" && l.SpeakerRole != "target" {
			t.Fatalf("unexpected speaker role: %s", l.SpeakerRole)
		}
	}
}

func TestGenerate_VariableSubstitution(t *testing.T) {
	vars := dialogue.Vars{
		AgentName:  "AGENT_A",
		TargetName: "AGENT_B",
		CropName:   "CROP_X",
		Season:     "summer",
	}

	// Run multiple times to cover random template picks
	for i := 0; i < 20; i++ {
		lines := dialogue.Generate("visit", "friend", 8, vars)
		for _, l := range lines {
			if strings.Contains(l.Text, "{agentName}") ||
				strings.Contains(l.Text, "{targetName}") ||
				strings.Contains(l.Text, "{cropName}") ||
				strings.Contains(l.Text, "{season}") {
				t.Fatalf("unreplaced placeholder in: %s", l.Text)
			}
		}
	}
}

func TestGenerate_AllScenes(t *testing.T) {
	vars := dialogue.Vars{AgentName: "A", TargetName: "B", CropName: "C", Season: "autumn"}
	scenes := []string{"visit", "trade", "help", "gift"}
	levels := []string{"stranger", "acquaintance", "friend", "close_friend", "best_friend"}

	for _, scene := range scenes {
		for _, level := range levels {
			lines := dialogue.Generate(scene, level, 5, vars)
			if len(lines) == 0 {
				t.Errorf("scene=%s level=%s returned 0 lines", scene, level)
			}
		}
	}
}

func TestGenerate_UnknownSceneFallback(t *testing.T) {
	vars := dialogue.Vars{AgentName: "X", TargetName: "Y", CropName: "Z", Season: "winter"}
	lines := dialogue.Generate("unknown_scene", "stranger", 5, vars)
	if len(lines) == 0 {
		t.Fatal("expected fallback lines for unknown scene")
	}
}

func TestGenerate_WarmVsCool(t *testing.T) {
	vars := dialogue.Vars{AgentName: "A", TargetName: "B", CropName: "C", Season: "spring"}

	// Warm (extroversion=8): first caller line should typically be more enthusiastic
	warmLines := dialogue.Generate("visit", "stranger", 8, vars)
	coolLines := dialogue.Generate("visit", "stranger", 3, vars)

	if len(warmLines) == 0 || len(coolLines) == 0 {
		t.Fatal("no lines returned")
	}
	// Both should have caller as first speaker
	if warmLines[0].SpeakerRole != "caller" {
		t.Errorf("warm: first speaker should be caller, got %s", warmLines[0].SpeakerRole)
	}
	if coolLines[0].SpeakerRole != "caller" {
		t.Errorf("cool: first speaker should be caller, got %s", coolLines[0].SpeakerRole)
	}
}
