package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	WeChat   WeChatConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port       string
	Mode       string // "debug", "release", "test"
	SpritesDir string // directory served at /sprites for character sprites
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	DSN string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// JWTConfig holds JWT signing settings.
type JWTConfig struct {
	Secret       string
	ExpiresHours int
}

// WeChatConfig holds WeChat miniprogram credentials.
type WeChatConfig struct {
	AppID     string
	AppSecret string
}

// Load reads configuration from environment variables (prefixed with APP_) and
// returns a populated Config struct. All values have sensible defaults for local
// development so the server can start without any environment set-up.
func Load() (*Config, error) {
	v := viper.New()

	// Allow environment variables to override any setting.
	// Supports both prefixed (APP_DATABASE_DSN) and plain (DATABASE_DSN) forms.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// Explicit bindings for the plain (no-prefix) variable names used in .env
	_ = v.BindEnv("server.port",       "SERVER_PORT")
	_ = v.BindEnv("server.mode",       "SERVER_MODE")
	_ = v.BindEnv("server.spritesdir", "SPRITES_DIR")
	_ = v.BindEnv("database.dsn",    "DATABASE_DSN")
	_ = v.BindEnv("redis.addr",      "REDIS_ADDR")
	_ = v.BindEnv("redis.password",  "REDIS_PASSWORD")
	_ = v.BindEnv("redis.db",        "REDIS_DB")
	_ = v.BindEnv("jwt.secret",      "JWT_SECRET")
	_ = v.BindEnv("wechat.appid",    "WECHAT_APP_ID")
	_ = v.BindEnv("wechat.appsecret","WECHAT_APP_SECRET")

	// ---- defaults ----
	v.SetDefault("server.port", "9080")
	v.SetDefault("server.mode", "debug")
	// Default sprite path (relative to server/ dir) for local development.
	// Override via SPRITES_DIR env var for production / Docker deployments.
	v.SetDefault("server.spritesdir", "../client/assets/sprites/characters/roles")

	v.SetDefault("database.dsn", "file:dev.db?cache=shared&_fk=1")

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	v.SetDefault("jwt.secret", "change-me-in-production")
	v.SetDefault("jwt.expireShours", 72)

	v.SetDefault("wechat.appid", "")
	v.SetDefault("wechat.appsecret", "")

	cfg := &Config{
		Server: ServerConfig{
			Port:       v.GetString("server.port"),
			Mode:       v.GetString("server.mode"),
			SpritesDir: v.GetString("server.spritesdir"),
		},
		Database: DatabaseConfig{
			DSN: v.GetString("database.dsn"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("redis.addr"),
			Password: v.GetString("redis.password"),
			DB:       v.GetInt("redis.db"),
		},
		JWT: JWTConfig{
			Secret:       v.GetString("jwt.secret"),
			ExpiresHours: v.GetInt("jwt.expireShours"),
		},
		WeChat: WeChatConfig{
			AppID:     v.GetString("wechat.appid"),
			AppSecret: v.GetString("wechat.appsecret"),
		},
	}

	return cfg, nil
}
