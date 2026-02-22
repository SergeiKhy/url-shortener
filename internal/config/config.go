package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig
	DB        DBConfig
	Redis     RedisConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Port string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type RedisConfig struct {
	Host string
	Port string
}

type AuthConfig struct {
	APIKeys map[string]string // API key -> name/description
}

type RateLimitConfig struct {
	RequestsPerSecond float64
	BurstSize         int
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	cfg.App.Port = viper.GetString("APP_PORT")
	cfg.DB.Host = viper.GetString("DB_HOST")
	cfg.DB.Port = viper.GetString("DB_PORT")
	cfg.DB.User = viper.GetString("DB_USER")
	cfg.DB.Password = viper.GetString("DB_PASSWORD")
	cfg.DB.Name = viper.GetString("DB_NAME")
	cfg.Redis.Host = viper.GetString("REDIS_HOST")
	cfg.Redis.Port = viper.GetString("REDIS_PORT")

	// Auth config - parse API keys from comma-separated string
	// Format: key1:name1,key2:name2
	apiKeysRaw := viper.GetString("API_KEYS")
	cfg.Auth.APIKeys = parseAPIKeys(apiKeysRaw)

	// Rate limit config
	cfg.RateLimit.RequestsPerSecond = viper.GetFloat64("RATE_LIMIT_RPS")
	if cfg.RateLimit.RequestsPerSecond == 0 {
		cfg.RateLimit.RequestsPerSecond = 10
	}
	cfg.RateLimit.BurstSize = viper.GetInt("RATE_LIMIT_BURST")
	if cfg.RateLimit.BurstSize == 0 {
		cfg.RateLimit.BurstSize = 20
	}

	return &cfg, nil
}

// parseAPIKeys parses comma-separated API keys in format "key1:name1,key2:name2"
func parseAPIKeys(raw string) map[string]string {
	keys := make(map[string]string)
	if raw == "" {
		return keys
	}

	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			keys[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return keys
}
