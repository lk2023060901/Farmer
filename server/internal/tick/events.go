package tick

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// eventTickProbability is the per-tick probability of attempting random events.
// 0.001 means roughly 1 tick in 1000 (~16 min) triggers the check — good for testing.
const eventTickProbability = 0.001

// runRandomEvents rolls the dice and triggers random events for a sample of farms.
func (e *Engine) runRandomEvents(ctx context.Context) {
	if rand.Float64() >= eventTickProbability {
		return
	}

	farms, err := e.db.Farm.Query().All(ctx)
	if err != nil {
		log.Printf("[events] query farms: %v", err)
		return
	}

	for _, farm := range farms {
		if rand.Float64() >= 0.3 {
			continue // only ~30% of farms get an event when outer check passes
		}
		kind := rand.Intn(7)
		e.triggerEvent(ctx, farm.OwnerID, kind)
	}
}

// triggerEvent dispatches a random event notification to a farm owner.
func (e *Engine) triggerEvent(ctx context.Context, ownerID uuid.UUID, kind int) {
	type eventDef struct{ title, content string }
	events := []eventDef{
		{"流浪商人到访", "神秘商人在村口摆摊，据说有稀有种子！去集市看看吧"},
		{"暴风雨来袭！", "农场遭受暴风雨，部分作物受到了损伤"},
		{"动物走失", "有一只动物不见了！赶紧发帖请邻居帮忙寻找吧"},
		{"获得神秘种子", "风吹来了3粒神秘种子，快去仓库查收吧"},
		{"害虫侵袭！", "农场出现害虫，可以向好友借农药来消灭它们"},
		{"彩虹出现！", "今天运气特别好，所有收获都有品质加成"},
		{"助手闹脾气了", "AI小助手今天和邻居闹别扭了，社交活跃度有所下降"},
	}
	if kind >= len(events) {
		kind = 0
	}
	ev := events[kind]
	_, err := e.db.Notification.Create().
		SetUserID(ownerID).
		SetType("random_event").
		SetTitle(ev.title).
		SetContent(ev.content).
		SetIsRead(false).
		SetCreatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		log.Printf("[events] create notification: %v", err)
	}
}

