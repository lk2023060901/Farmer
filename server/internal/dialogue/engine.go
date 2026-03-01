// Package dialogue implements the template-based dialogue engine.
//
// Scenes: visit, trade, help, gift
// Relationship levels: stranger, acquaintance, friend, close_friend, best_friend
// Variable substitution: {agentName}, {targetName}, {cropName}, {season}
//
// Each visit produces a 2-4 turn exchange stored as []Line.
package dialogue

import (
	"math/rand"
	"strings"
)

// Line is one turn in the conversation.
type Line struct {
	SpeakerRole string // "caller" | "target"
	Text        string
}

// Vars holds substitution values for template rendering.
type Vars struct {
	AgentName  string // visiting agent's name
	TargetName string // host's name
	CropName   string // most recently relevant crop
	Season     string // spring/summer/autumn/winter
}

// Generate returns a multi-turn (2-4 lines) dialogue for the given scene and
// relationship level. The personality extroversion value (1-10) biases which
// template set is selected (high extroversion → warmer openers).
func Generate(scene, relLevel string, extroversion int, v Vars) []Line {
	templates := selectTemplates(scene, relLevel, extroversion)
	if len(templates) == 0 {
		return fallback(v)
	}
	// Pick a random template
	t := templates[rand.Intn(len(templates))]
	result := make([]Line, len(t))
	for i, l := range t {
		result[i] = Line{
			SpeakerRole: l.SpeakerRole,
			Text:        substitute(l.Text, v),
		}
	}
	return result
}

// substitute replaces {key} placeholders in text with their values.
func substitute(text string, v Vars) string {
	r := strings.NewReplacer(
		"{agentName}", v.AgentName,
		"{targetName}", v.TargetName,
		"{cropName}", v.CropName,
		"{season}", seasonCN(v.Season),
	)
	return r.Replace(text)
}

func seasonCN(s string) string {
	switch s {
	case "spring":
		return "春天"
	case "summer":
		return "夏天"
	case "autumn":
		return "秋天"
	case "winter":
		return "冬天"
	default:
		return s
	}
}

func fallback(v Vars) []Line {
	return []Line{
		{SpeakerRole: "caller", Text: substitute("嗨，{targetName}！我是{agentName}，来串门啦～", v)},
		{SpeakerRole: "target", Text: substitute("欢迎欢迎，{agentName}！", v)},
	}
}

// selectTemplates returns the candidate template list for given scene + level + personality.
func selectTemplates(scene, relLevel string, extroversion int) [][]Line {
	switch scene {
	case "visit":
		return visitTemplates(relLevel, extroversion)
	case "trade":
		return tradeTemplates(relLevel)
	case "help":
		return helpTemplates(relLevel)
	case "gift":
		return giftTemplates(relLevel)
	}
	return nil
}

// ── Visit templates ────────────────────────────────────────────────────────

func visitTemplates(relLevel string, extroversion int) [][]Line {
	// Warm openers for extroversion >= 6, cool for <= 5
	if extroversion >= 6 {
		return warmVisitTemplates(relLevel)
	}
	return coolVisitTemplates(relLevel)
}

func warmVisitTemplates(relLevel string) [][]Line {
	base := [][]Line{
		{
			{SpeakerRole: "caller", Text: "嗨嗨！{targetName}，我是{agentName}，来你农场取取经～"},
			{SpeakerRole: "target", Text: "哇，{agentName}来啦！快进来快进来！"},
			{SpeakerRole: "caller", Text: "你的{cropName}长得好漂亮，有啥秘诀吗？"},
			{SpeakerRole: "target", Text: "没啥秘诀，就是每天勤浇水哈哈～"},
		},
		{
			{SpeakerRole: "caller", Text: "嗯哼～{targetName}，{season}的农场好美啊！"},
			{SpeakerRole: "target", Text: "谢谢夸奖！{agentName}你的农场肯定也不差！"},
			{SpeakerRole: "caller", Text: "哪里哪里，向你多学习！"},
		},
		{
			{SpeakerRole: "caller", Text: "嘿，{targetName}！我顺路来看看你！"},
			{SpeakerRole: "target", Text: "真巧，{agentName}！我刚收完{cropName}，累得很。"},
			{SpeakerRole: "caller", Text: "哇，丰收了！要不要我帮你浇水？"},
			{SpeakerRole: "target", Text: "不用啦，下次你来我帮你！"},
		},
	}

	switch relLevel {
	case "friend", "close_friend", "best_friend":
		base = append(base, [][]Line{
			{
				{SpeakerRole: "caller", Text: "老朋友！{targetName}，好久不来串门了，想死你了！"},
				{SpeakerRole: "target", Text: "哈哈，{agentName}你最近忙啥呢？"},
				{SpeakerRole: "caller", Text: "忙着种{cropName}，忙到忘时间了～"},
				{SpeakerRole: "target", Text: "懂懂，一起奋斗！加油！"},
			},
		}...)
	}
	return base
}

func coolVisitTemplates(relLevel string) [][]Line {
	base := [][]Line{
		{
			{SpeakerRole: "caller", Text: "……你好，{targetName}。我是{agentName}，来看看。"},
			{SpeakerRole: "target", Text: "哦，{agentName}啊，进来吧。"},
			{SpeakerRole: "caller", Text: "你的{cropName}不错。"},
			{SpeakerRole: "target", Text: "嗯，谢谢。"},
		},
		{
			{SpeakerRole: "caller", Text: "你好，{targetName}。{season}了，来串个门。"},
			{SpeakerRole: "target", Text: "嗯，欢迎。有事吗？"},
			{SpeakerRole: "caller", Text: "没事，就是路过。"},
		},
		{
			{SpeakerRole: "caller", Text: "{targetName}，我是{agentName}，想参观一下你的农场。"},
			{SpeakerRole: "target", Text: "可以啊，随便看。"},
			{SpeakerRole: "caller", Text: "{cropName}种得很整齐。"},
			{SpeakerRole: "target", Text: "还行吧，凑合。"},
		},
	}
	if relLevel == "stranger" {
		return base
	}
	// acquaintance and above — slightly warmer
	base = append(base, [][]Line{
		{
			{SpeakerRole: "caller", Text: "嗯，{targetName}……最近{cropName}怎么样了？"},
			{SpeakerRole: "target", Text: "还可以，你的呢，{agentName}？"},
			{SpeakerRole: "caller", Text: "差不多，慢慢来吧。"},
		},
	}...)
	return base
}

// ── Trade templates ────────────────────────────────────────────────────────

func tradeTemplates(relLevel string) [][]Line {
	base := [][]Line{
		{
			{SpeakerRole: "caller", Text: "嗨，{targetName}！想买你的{cropName}，有货吗？"},
			{SpeakerRole: "target", Text: "有的有的，{agentName}，给你个好价钱！"},
			{SpeakerRole: "caller", Text: "太好了，成交！"},
		},
		{
			{SpeakerRole: "caller", Text: "{targetName}，你这批{cropName}品质真好，给我留一些？"},
			{SpeakerRole: "target", Text: "看在{agentName}你的面上，当然！"},
		},
		{
			{SpeakerRole: "caller", Text: "我这有多余的{cropName}，{targetName}，要不要换？"},
			{SpeakerRole: "target", Text: "诶，正好需要！怎么换法？"},
			{SpeakerRole: "caller", Text: "公平交换，你看行不？"},
			{SpeakerRole: "target", Text: "行，就这么办！"},
		},
	}
	if relLevel == "close_friend" || relLevel == "best_friend" {
		base = append(base, [][]Line{
			{
				{SpeakerRole: "caller", Text: "老伙计，{targetName}！帮我留点{cropName}，我下次来拿！"},
				{SpeakerRole: "target", Text: "包在我身上，{agentName}，朋友嘛！"},
			},
		}...)
	}
	return base
}

// ── Help templates ─────────────────────────────────────────────────────────

func helpTemplates(relLevel string) [][]Line {
	return [][]Line{
		{
			{SpeakerRole: "caller", Text: "{targetName}，我的{cropName}今天没人照顾，能帮个忙吗？"},
			{SpeakerRole: "target", Text: "当然可以，{agentName}，交给我！"},
			{SpeakerRole: "caller", Text: "真的太感谢了！"},
		},
		{
			{SpeakerRole: "caller", Text: "我出门了，{targetName}，能帮我看着点农场吗？"},
			{SpeakerRole: "target", Text: "没问题，{agentName}，放心去吧！"},
		},
		{
			{SpeakerRole: "caller", Text: "唉，{cropName}快枯萎了，{targetName}来不及浇水，救救我！"},
			{SpeakerRole: "target", Text: "哎呀，{agentName}快别急，我来帮你！"},
			{SpeakerRole: "caller", Text: "谢谢谢谢！下次我请你吃饭！"},
			{SpeakerRole: "target", Text: "哈哈，请饭就不用了，大家互帮互助嘛！"},
		},
	}
}

// ── Gift templates ─────────────────────────────────────────────────────────

func giftTemplates(relLevel string) [][]Line {
	base := [][]Line{
		{
			{SpeakerRole: "caller", Text: "嘿，{targetName}！送你一些{cropName}，自己种的！"},
			{SpeakerRole: "target", Text: "哇，{agentName}你太客气了！"},
			{SpeakerRole: "caller", Text: "不客气，大家分享嘛～"},
		},
		{
			{SpeakerRole: "caller", Text: "{targetName}，今天{season}收成不错，多的{cropName}送你尝尝！"},
			{SpeakerRole: "target", Text: "太好了，{agentName}，我最喜欢{cropName}了！"},
			{SpeakerRole: "caller", Text: "喜欢就好～"},
			{SpeakerRole: "target", Text: "下次我也送你！"},
		},
	}
	if relLevel == "best_friend" {
		base = append(base, [][]Line{
			{
				{SpeakerRole: "caller", Text: "最好的朋友，{targetName}！这一箱{cropName}是给你的！"},
				{SpeakerRole: "target", Text: "{agentName}！！太感动了，我爱你！"},
				{SpeakerRole: "caller", Text: "哈哈，友谊万岁！"},
			},
		}...)
	}
	return base
}
