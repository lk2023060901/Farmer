package llm

import "testing"

func TestPickTemplate_fallsBackToGeneral(t *testing.T) {
	// Unknown scene → should not panic, should return something
	tmpl := PickTemplate("unknown_scene", RelationStranger, 0)
	if tmpl.LineA == "" || tmpl.LineB == "" {
		t.Errorf("expected non-empty template, got %+v", tmpl)
	}
}

func TestRenderTemplate_substitutesNames(t *testing.T) {
	tmpl := dialogTemplate{LineA: "{name_a}：你好。", LineB: "{name_b}：你好！"}
	lineA, lineB := RenderTemplate(tmpl, "小明", "小红")
	if lineA != "小明：你好。" {
		t.Errorf("lineA = %q, want %q", lineA, "小明：你好。")
	}
	if lineB != "小红：你好！" {
		t.Errorf("lineB = %q, want %q", lineB, "小红：你好！")
	}
}

func TestSafetyFilter(t *testing.T) {
	if !SafetyFilter("今天天气真好！") {
		t.Error("expected clean content to pass filter")
	}
	if SafetyFilter("这是违法行为") {
		t.Error("expected blocked content to fail filter")
	}
}

func TestDialogFingerprint_deterministic(t *testing.T) {
	req := DialogRequest{
		AgentA:      AgentPersonality{Extroversion: 7, Generosity: 5, Adventure: 3},
		AgentB:      AgentPersonality{Extroversion: 4, Generosity: 8, Adventure: 6},
		RelationLvl: RelationFriend,
		Scene:       SceneVisit,
	}
	fp1 := dialogFingerprint(req)
	fp2 := dialogFingerprint(req)
	if fp1 != fp2 {
		t.Errorf("fingerprint is not deterministic: %q != %q", fp1, fp2)
	}
	if len(fp1) != 16 {
		t.Errorf("expected 16-char fingerprint, got %d", len(fp1))
	}
}

func TestAffinityToLevel(t *testing.T) {
	cases := []struct {
		affinity int
		want     RelationLevel
	}{
		{0, RelationStranger},
		{19, RelationStranger},
		{20, RelationAcquaintance},
		{49, RelationAcquaintance},
		{50, RelationFriend},
		{79, RelationFriend},
		{80, RelationBestFriend},
		{100, RelationBestFriend},
	}
	for _, tc := range cases {
		got := AffinityToLevel(tc.affinity)
		if got != tc.want {
			t.Errorf("AffinityToLevel(%d) = %q, want %q", tc.affinity, got, tc.want)
		}
	}
}
