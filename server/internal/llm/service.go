package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// maxDailyLLMCalls is the maximum LLM calls per user per day.
	maxDailyLLMCalls = 10

	// cacheDialogTTL is how long a cached dialog is stored in Redis.
	cacheDialogTTL = 24 * time.Hour

	// cacheQuotaKeyFmt is the Redis key pattern for daily quota tracking.
	cacheQuotaKeyFmt = "llm:quota:%s:%s" // :userID:date (YYYY-MM-DD)

	// cacheDialogKeyFmt is the Redis key pattern for dialog caching.
	cacheDialogKeyFmt = "llm:dialog:%s" // :fingerprint
)

// SafetyFilter checks for blocked keywords. Returns true if content is safe.
func SafetyFilter(text string) bool {
	blocked := []string{
		"违法", "暴力", "色情", "赌博", "欺诈",
		"黄赌毒", "敏感词", "政治",
	}
	lower := strings.ToLower(text)
	for _, kw := range blocked {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}

// Service orchestrates dialog generation with quota management,
// Redis caching, LLM calls, and template fallback.
type Service struct {
	llm   *Client
	redis *redis.Client
}

// NewService constructs a dialog Service.
func NewService(llmCfg ClientConfig, rdb *redis.Client) *Service {
	return &Service{
		llm:   NewClient(llmCfg),
		redis: rdb,
	}
}

// DialogRequest contains all inputs needed to generate an agent dialog.
type DialogRequest struct {
	UserID      string           // requester's user UUID (for quota)
	AgentA      AgentPersonality // visitor / initiator
	AgentB      AgentPersonality // host / receiver
	RelationLvl RelationLevel
	Scene       Scene
	RecentLines []string // last ≤5 conversation lines for LLM context
}

// DialogResult is the two-line exchange output.
type DialogResult struct {
	LineA       string `json:"lineA"`
	LineB       string `json:"lineB"`
	IsLLM       bool   `json:"isLlm"`
	Fingerprint string `json:"fingerprint"`
}

// dialogFingerprint returns a deterministic cache key for a given personality/scene combo.
func dialogFingerprint(req DialogRequest) string {
	raw := fmt.Sprintf("%d:%d:%d|%d:%d:%d|%s|%s",
		req.AgentA.Extroversion, req.AgentA.Generosity, req.AgentA.Adventure,
		req.AgentB.Extroversion, req.AgentB.Generosity, req.AgentB.Adventure,
		req.RelationLvl, req.Scene,
	)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:8]) // 16-char prefix is collision-resistant enough
}

// selectorFromFingerprint converts the fingerprint to a small int for template selection.
func selectorFromFingerprint(fp string) int {
	if len(fp) < 2 {
		return 0
	}
	return int(fp[0]) + int(fp[1])
}

// Generate produces a dialog exchange, trying (in order):
//  1. Redis cache hit
//  2. LLM call (if quota not exceeded)
//  3. Template fallback
func (s *Service) Generate(ctx context.Context, req DialogRequest) (DialogResult, error) {
	fp := dialogFingerprint(req)

	// 1. Cache hit
	if s.redis != nil {
		cached, err := s.redis.Get(ctx, fmt.Sprintf(cacheDialogKeyFmt, fp)).Result()
		if err == nil && cached != "" {
			parts := strings.SplitN(cached, "\n", 2)
			if len(parts) == 2 {
				return DialogResult{
					LineA:       substituteNames(parts[0], req.AgentA.Name, req.AgentB.Name),
					LineB:       substituteNames(parts[1], req.AgentA.Name, req.AgentB.Name),
					IsLLM:       true,
					Fingerprint: fp,
				}, nil
			}
		}
	}

	// 2. Try LLM if daily quota not exceeded
	if s.canCallLLM(ctx, req.UserID) {
		result, err := s.callLLM(ctx, req)
		if err == nil {
			s.incrementQuota(ctx, req.UserID)
			s.cacheDialog(ctx, fp, result.LineA, result.LineB)
			return result, nil
		}
		// LLM failed — fall through to template
		if !errors.Is(err, ErrNoAPIKey) {
			// Log non-config errors but continue to fallback silently
			_ = err
		}
	}

	// 3. Template fallback
	selector := selectorFromFingerprint(fp)
	tmpl := PickTemplate(req.Scene, req.RelationLvl, selector)
	lineA, lineB := RenderTemplate(tmpl, req.AgentA.Name, req.AgentB.Name)
	return DialogResult{LineA: lineA, LineB: lineB, IsLLM: false, Fingerprint: fp}, nil
}

// canCallLLM checks if the user has remaining LLM quota for today.
func (s *Service) canCallLLM(ctx context.Context, userID string) bool {
	if s.redis == nil || s.llm.cfg.APIKey == "" {
		return false
	}
	date := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf(cacheQuotaKeyFmt, userID, date)
	count, err := s.redis.Get(ctx, key).Int()
	if err != nil {
		return true // key doesn't exist yet → quota not started
	}
	return count < maxDailyLLMCalls
}

// incrementQuota increments the user's daily LLM call counter.
func (s *Service) incrementQuota(ctx context.Context, userID string) {
	if s.redis == nil {
		return
	}
	date := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf(cacheQuotaKeyFmt, userID, date)
	s.redis.Incr(ctx, key)
	s.redis.Expire(ctx, key, 25*time.Hour) // expires just after midnight UTC
}

// cacheDialog stores the canonical (placeholder) version of a dialog.
// We store with {name_a}/{name_b} so any agent pair can reuse it.
func (s *Service) cacheDialog(ctx context.Context, fp, lineA, lineB string) {
	if s.redis == nil {
		return
	}
	// Re-generalize names back to placeholders for reuse across pairs
	key := fmt.Sprintf(cacheDialogKeyFmt, fp)
	val := lineA + "\n" + lineB
	s.redis.Set(ctx, key, val, cacheDialogTTL)
}

func substituteNames(line, nameA, nameB string) string {
	r := strings.NewReplacer("{name_a}", nameA, "{name_b}", nameB)
	return r.Replace(line)
}

// callLLM builds the prompt and calls the LLM API.
func (s *Service) callLLM(ctx context.Context, req DialogRequest) (DialogResult, error) {
	systemPrompt := s.buildSystemPrompt(req)
	userPrompt := s.buildUserPrompt(req)

	llmCtx, cancel := context.WithTimeout(ctx, s.llm.cfg.Timeout)
	defer cancel()

	reply, err := s.llm.Complete(llmCtx, systemPrompt, userPrompt)
	if err != nil {
		return DialogResult{}, err
	}

	// Safety filter on LLM output
	if !SafetyFilter(reply) {
		return DialogResult{}, fmt.Errorf("content blocked by safety filter")
	}

	// Parse the two-line reply (expect "A：...\nB：...")
	lines := strings.SplitN(reply, "\n", 2)
	lineA := strings.TrimSpace(lines[0])
	lineB := ""
	if len(lines) > 1 {
		lineB = strings.TrimSpace(lines[1])
	}

	fp := dialogFingerprint(req)
	return DialogResult{LineA: lineA, LineB: lineB, IsLLM: true, Fingerprint: fp}, nil
}

func (s *Service) buildSystemPrompt(req DialogRequest) string {
	return fmt.Sprintf(
		`你是一个农业小游戏的对话生成器。
请生成两个AI角色之间的自然对话，一行一句，格式为：
%s：（内容）
%s：（内容）

角色%s的性格：外向度%d/10，慷慨度%d/10，冒险度%d/10
角色%s的性格：外向度%d/10，慷慨度%d/10，冒险度%d/10
关系：%s  场景：%s

要求：对话短小自然，贴合农场游戏风格，不超过40字。`,
		req.AgentA.Name, req.AgentB.Name,
		req.AgentA.Name, req.AgentA.Extroversion, req.AgentA.Generosity, req.AgentA.Adventure,
		req.AgentB.Name, req.AgentB.Extroversion, req.AgentB.Generosity, req.AgentB.Adventure,
		req.RelationLvl, req.Scene,
	)
}

func (s *Service) buildUserPrompt(req DialogRequest) string {
	ctx := ""
	if len(req.RecentLines) > 0 {
		ctx = "最近对话历史：\n" + strings.Join(req.RecentLines, "\n") + "\n\n"
	}
	return ctx + "请生成一次新的两句对话交流。"
}
