// Package llm provides LLM-powered dialog generation with template fallback,
// Redis caching, daily quota management, and content safety filtering.
package llm

import (
	"fmt"
	"strings"
)

// Scene represents the context/scene type for a dialog.
type Scene string

const (
	SceneVisit  Scene = "visit"
	SceneTrade  Scene = "trade"
	SceneHelp   Scene = "help"
	SceneGift   Scene = "gift"
	SceneGeneral Scene = "general"
)

// AgentPersonality captures the relevant personality dimensions for dialog.
type AgentPersonality struct {
	Name        string
	Extroversion int // 1-10: introverted → extroverted
	Generosity   int // 1-10: frugal → generous
	Adventure    int // 1-10: conservative → adventurous
}

// RelationLevel maps affinity score to a named relationship tier.
type RelationLevel string

const (
	RelationStranger    RelationLevel = "stranger"    // < 20
	RelationAcquaintance RelationLevel = "acquaintance" // 20-49
	RelationFriend      RelationLevel = "friend"      // 50-79
	RelationBestFriend  RelationLevel = "bestfriend"  // 80+
)

func AffinityToLevel(affinity int) RelationLevel {
	switch {
	case affinity >= 80:
		return RelationBestFriend
	case affinity >= 50:
		return RelationFriend
	case affinity >= 20:
		return RelationAcquaintance
	default:
		return RelationStranger
	}
}

// templateVars are substituted into template strings.
type templateVars struct {
	NameA string
	NameB string
}

func substitute(tmpl string, v templateVars) string {
	r := strings.NewReplacer(
		"{name_a}", v.NameA,
		"{name_b}", v.NameB,
	)
	return r.Replace(tmpl)
}

// dialogTemplate is a pair of lines for a two-agent exchange.
type dialogTemplate struct {
	LineA string // spoken by agent A (visitor / initiator)
	LineB string // spoken by agent B (host / receiver)
}

// fallbackTemplates maps (scene, relation) to a list of dialog templates.
// Callers pick one by hashing agent personalities for consistent variety.
var fallbackTemplates = map[string][]dialogTemplate{
	key(SceneVisit, RelationStranger): {
		{"{name_a}：你好，我是来参观一下你的农场的。", "{name_b}：欢迎欢迎，随便看看！"},
		{"{name_a}：第一次来，农场好漂亮呀！", "{name_b}：哈哈，还行吧，多来玩！"},
	},
	key(SceneVisit, RelationAcquaintance): {
		{"{name_a}：又来打扰啦，看看有什么新作物没有？", "{name_b}：来了来了！今天刚种了一批草莓。"},
		{"{name_a}：最近忙不忙？来串串门。", "{name_b}：不忙不忙，一起聊聊呗！"},
	},
	key(SceneVisit, RelationFriend): {
		{"{name_a}：好久不见！最近收成怎么样？", "{name_b}：还不错哈，正想找你交流种植心得呢！"},
		{"{name_a}：哎，我带了些种子来，送你！", "{name_b}：哇谢谢，你真好，我也有东西回赠你！"},
	},
	key(SceneVisit, RelationBestFriend): {
		{"{name_a}：来啦来啦！{name_b}，今天帮我浇个水吧？", "{name_b}：没问题，就知道你要来找我！"},
		{"{name_a}：{name_b}，你快来看，我发现一种新的培育技巧！", "{name_b}：真的？快说！我们一起研究！"},
	},
	key(SceneTrade, RelationStranger): {
		{"{name_a}：请问这个怎么卖？", "{name_b}：这批是10金一个，质量不错的。"},
	},
	key(SceneTrade, RelationAcquaintance): {
		{"{name_a}：我想换一些你的作物，你有意愿吗？", "{name_b}：可以啊，看看你有什么吧。"},
	},
	key(SceneTrade, RelationFriend): {
		{"{name_a}：老朋友，给我便宜点！", "{name_b}：就你，行，给你打个八折。"},
	},
	key(SceneTrade, RelationBestFriend): {
		{"{name_a}：这些你拿去用吧，不用给钱。", "{name_b}：哎你太客气了！下次我请客！"},
	},
	key(SceneHelp, RelationStranger): {
		{"{name_a}：能帮我浇个水吗？体力不够了。", "{name_b}：好的，帮你一下。"},
	},
	key(SceneHelp, RelationAcquaintance): {
		{"{name_a}：{name_b}，最近我种子不够用了，能借我一些吗？", "{name_b}：没问题，我这里有些多余的玉米种，给你拿一些。"},
	},
	key(SceneHelp, RelationFriend): {
		{"{name_a}：{name_b}，农场临时有急事，帮我看两天？", "{name_b}：交给我！放心去吧，保证弄好。"},
	},
	key(SceneHelp, RelationBestFriend): {
		{"{name_a}：出去旅行了，麻烦你帮我代管农场三天！", "{name_b}：早说嘛！我替你盯着，好好玩！"},
	},
	key(SceneGift, RelationAcquaintance): {
		{"{name_a}：这个送你，一点心意。", "{name_b}：哦？谢谢你，太客气了！"},
	},
	key(SceneGift, RelationFriend): {
		{"{name_a}：这是我刚收获的蜂蜜，特意给你留的！", "{name_b}：哇！谢谢{name_a}，你真是太好了，我很感动！"},
	},
	key(SceneGift, RelationBestFriend): {
		{"{name_a}：今天是你的生日！我给你准备了礼物。", "{name_b}：{name_a}你还记得！！太开心了，谢谢你！"},
	},
	key(SceneGeneral, RelationStranger): {
		{"{name_a}：你好。", "{name_b}：你好，有什么事吗？"},
	},
	key(SceneGeneral, RelationFriend): {
		{"{name_a}：最近农场怎么样？", "{name_b}：还不错，就是天气有点干，需要多浇水。"},
	},
}

func key(scene Scene, level RelationLevel) string {
	return fmt.Sprintf("%s:%s", scene, level)
}

// PickTemplate picks a fallback template deterministically based on a selector (0-based index).
// Falls back to SceneGeneral/RelationStranger if no matching template exists.
func PickTemplate(scene Scene, level RelationLevel, selector int) dialogTemplate {
	k := key(scene, level)
	tmplList, ok := fallbackTemplates[k]
	if !ok || len(tmplList) == 0 {
		// Try general scene with same relation
		k = key(SceneGeneral, level)
		tmplList = fallbackTemplates[k]
	}
	if len(tmplList) == 0 {
		return dialogTemplate{
			LineA: "{name_a}：你好。",
			LineB: "{name_b}：你好！",
		}
	}
	return tmplList[selector%len(tmplList)]
}

// RenderTemplate substitutes {name_a} and {name_b} in the template.
func RenderTemplate(t dialogTemplate, nameA, nameB string) (lineA, lineB string) {
	v := templateVars{NameA: nameA, NameB: nameB}
	return substitute(t.LineA, v), substitute(t.LineB, v)
}
