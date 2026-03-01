// Package router integration tests exercise the full HTTP stack against a real
// PostgreSQL database. They require DATABASE_DSN to be set (or will skip).
//
// Run with:
//
//	DATABASE_DSN="postgres://farmer:farmer_secret@localhost:5433/farmer_dev?sslmode=disable" \
//	  go test ./internal/router/... -v -run TestIntegration
package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liukai/farmer/server/config"
	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/internal/llm"
	"github.com/liukai/farmer/server/internal/store"
	"github.com/liukai/farmer/server/internal/ws"

	_ "github.com/lib/pq"
)

// testApp holds the test server and auth token.
type testApp struct {
	server *httptest.Server
	token  string
}

// setupTestApp creates a real router connected to the dev database.
// It skips if DATABASE_DSN is not set.
func setupTestApp(t *testing.T) *testApp {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN not set — skipping integration tests")
	}

	cfg := &config.Config{
		Server:   config.ServerConfig{Mode: "test"},
		Database: config.DatabaseConfig{DSN: dsn},
		JWT:      config.JWTConfig{Secret: "test-secret", ExpiresHours: 24},
	}

	db, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	hub := ws.NewHub()
	llmSvc := llm.NewService(llm.DefaultClientConfig(), nil) // no Redis in test

	r := New(cfg, db, hub, llmSvc)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Dev-login to get a token
	token := devLogin(t, srv.URL)
	return &testApp{server: srv, token: token}
}

// devLogin calls /api/v1/auth/dev-login and returns the JWT.
func devLogin(t *testing.T, baseURL string) string {
	t.Helper()
	body := `{"openid":"integration-test-user-001"}`
	resp := doRequest(t, baseURL, "POST", "/api/v1/auth/dev-login", body, "")
	defer resp.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("dev-login decode: %v", err)
	}
	if result.Code != 0 {
		t.Fatalf("dev-login failed: code=%d", result.Code)
	}
	return result.Data.Token
}

// doRequest performs an HTTP request and returns the response.
func doRequest(t *testing.T, baseURL, method, path, body, token string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, baseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// decodeBody decodes JSON response body into the given target.
func decodeBody(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestIntegration_HealthCheck(t *testing.T) {
	app := setupTestApp(t)
	resp := doRequest(t, app.server.URL, "GET", "/health", "", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health: got %d, want 200", resp.StatusCode)
	}
}

func TestIntegration_Auth_DevLogin(t *testing.T) {
	app := setupTestApp(t)
	if app.token == "" {
		t.Error("expected non-empty token")
	}
}

func TestIntegration_Users_GetMe(t *testing.T) {
	app := setupTestApp(t)
	resp := doRequest(t, app.server.URL, "GET", "/api/v1/users/me", "", app.token)
	var result struct {
		Code int `json:"code"`
		Data struct {
			Level   int    `json:"level"`
			Coins   int64  `json:"coins"`
			Stamina int    `json:"stamina"`
			ID      string `json:"id"`
		} `json:"data"`
	}
	decodeBody(t, resp, &result)
	if result.Code != 0 {
		t.Errorf("GetMe: code=%d", result.Code)
	}
	if result.Data.ID == "" {
		t.Error("GetMe: expected non-empty user ID")
	}
	if result.Data.Level < 1 {
		t.Errorf("GetMe: level=%d, want ≥ 1", result.Data.Level)
	}
}

func TestIntegration_Farm_GetMine(t *testing.T) {
	app := setupTestApp(t)
	resp := doRequest(t, app.server.URL, "GET", "/api/v1/farms/mine", "", app.token)
	var result struct {
		Code int `json:"code"`
		Data struct {
			ID    string `json:"id"`
			Plots []any  `json:"plots"`
		} `json:"data"`
	}
	decodeBody(t, resp, &result)
	if result.Code != 0 {
		t.Errorf("GetMine: code=%d", result.Code)
	}
	if result.Data.ID == "" {
		t.Error("GetMine: expected farm ID")
	}
}

func TestIntegration_Daily_StreakAndCheckin(t *testing.T) {
	app := setupTestApp(t)

	// Get streak
	resp := doRequest(t, app.server.URL, "GET", "/api/v1/daily/streak", "", app.token)
	var streakResult struct {
		Code int `json:"code"`
		Data struct {
			Streak  int  `json:"streak"`
			Rewards []any `json:"rewards"`
		} `json:"data"`
	}
	decodeBody(t, resp, &streakResult)
	if streakResult.Code != 0 {
		t.Errorf("GetStreak: code=%d", streakResult.Code)
	}
	if len(streakResult.Data.Rewards) != 7 {
		t.Errorf("GetStreak: expected 7 rewards, got %d", len(streakResult.Data.Rewards))
	}

	// Checkin (idempotent — may already be checked in)
	resp2 := doRequest(t, app.server.URL, "POST", "/api/v1/daily/checkin", "", app.token)
	var checkinResult struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Streak         int    `json:"streak"`
			RewardType     string `json:"rewardType"`
			RewardQuantity int    `json:"rewardQuantity"`
		} `json:"data"`
	}
	decodeBody(t, resp2, &checkinResult)
	if checkinResult.Code != 0 {
		t.Errorf("CheckIn: code=%d, message=%s", checkinResult.Code, checkinResult.Message)
	}
	if checkinResult.Data.Streak < 1 {
		t.Errorf("CheckIn: streak=%d, want ≥ 1", checkinResult.Data.Streak)
	}
	if checkinResult.Data.RewardType == "" {
		t.Error("CheckIn: expected non-empty rewardType")
	}
}

func TestIntegration_Tutorial_Progress(t *testing.T) {
	app := setupTestApp(t)

	// Get initial progress
	resp := doRequest(t, app.server.URL, "GET", "/api/v1/tutorial/progress", "", app.token)
	var progressResult struct {
		Code int `json:"code"`
		Data struct {
			CompletedSteps []string `json:"completedSteps"`
		} `json:"data"`
	}
	decodeBody(t, resp, &progressResult)
	if progressResult.Code != 0 {
		t.Errorf("GetProgress: code=%d", progressResult.Code)
	}

	// Complete a step
	stepName := fmt.Sprintf("test-step-%d", time.Now().UnixNano())
	body := fmt.Sprintf(`{"step":%q}`, stepName)
	resp2 := doRequest(t, app.server.URL, "POST", "/api/v1/tutorial/complete-step", body, app.token)
	var completeResult struct {
		Code int `json:"code"`
		Data struct {
			CompletedSteps []string `json:"completedSteps"`
		} `json:"data"`
	}
	decodeBody(t, resp2, &completeResult)
	if completeResult.Code != 0 {
		t.Errorf("CompleteStep: code=%d", completeResult.Code)
	}

	// Verify idempotency — same step again
	resp3 := doRequest(t, app.server.URL, "POST", "/api/v1/tutorial/complete-step", body, app.token)
	var idempotentResult struct {
		Code int `json:"code"`
	}
	decodeBody(t, resp3, &idempotentResult)
	if idempotentResult.Code != 0 {
		t.Errorf("CompleteStep idempotent: code=%d, want 0", idempotentResult.Code)
	}
}

func TestIntegration_Activity_OfflineSummary(t *testing.T) {
	app := setupTestApp(t)
	since := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	path := "/api/v1/activity/offline-summary?since=" + since
	resp := doRequest(t, app.server.URL, "GET", path, "", app.token)
	var result struct {
		Code int `json:"code"`
		Data struct {
			TotalEvents int    `json:"totalEvents"`
			Summary     string `json:"summary"`
		} `json:"data"`
	}
	decodeBody(t, resp, &result)
	if result.Code != 0 {
		t.Errorf("OfflineSummary: code=%d", result.Code)
	}
	if result.Data.Summary == "" {
		t.Error("OfflineSummary: expected non-empty summary string")
	}
}

func TestIntegration_Unauthorized_Rejected(t *testing.T) {
	app := setupTestApp(t)
	resp := doRequest(t, app.server.URL, "GET", "/api/v1/users/me", "", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token request: got %d, want 401", resp.StatusCode)
	}
}

func TestIntegration_InvalidToken_Rejected(t *testing.T) {
	app := setupTestApp(t)
	resp := doRequest(t, app.server.URL, "GET", "/api/v1/users/me", "", "invalid.token.here")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("invalid-token request: got %d, want 401", resp.StatusCode)
	}
}

// Verify that DB client is reachable from integration test
func TestIntegration_DbReachable(t *testing.T) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN not set")
	}
	client, err := ent.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer client.Close()
	// Ping via a simple query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.User.Query().Limit(1).All(ctx)
	if err != nil {
		t.Fatalf("db ping: %v", err)
	}
}
