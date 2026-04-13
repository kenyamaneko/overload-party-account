package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config は account サービスの起動設定を保持します。
type Config struct {
	Port        int
	Env         string
	DatabaseURL string

	PubsubProjectID             string
	FactionSelectedSubscription string
	PremiumUpdatedSubscription  string

	// FirestoreProjectID は game_config の読み取り先プロジェクト ID。
	// ローカル/CI では FIRESTORE_EMULATOR_HOST を別途設定することでエミュレーターに接続。
	FirestoreProjectID string
}

// FromEnv は環境変数から Config を構築します。
func FromEnv() (*Config, error) {
	cfg := &Config{
		Port:                        9005,
		Env:                         getEnv("ENV", "dev"),
		DatabaseURL:                 os.Getenv("DATABASE_URL"),
		PubsubProjectID:             os.Getenv("PUBSUB_PROJECT_ID"),
		FactionSelectedSubscription: getEnv("FACTION_SELECTED_SUBSCRIPTION", "faction-selected-account-sub"),
		PremiumUpdatedSubscription:  getEnv("PREMIUM_UPDATED_SUBSCRIPTION", "premium-updated-account-sub"),
		FirestoreProjectID:          os.Getenv("FIRESTORE_PROJECT_ID"),
	}

	if raw := os.Getenv("PORT"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("config: PORT %q: %w", raw, err)
		}
		cfg.Port = n
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.PubsubProjectID == "" {
		return nil, fmt.Errorf("config: PUBSUB_PROJECT_ID is required (account subscribes to faction-selected / premium-updated)")
	}
	if cfg.FirestoreProjectID == "" {
		return nil, fmt.Errorf("config: FIRESTORE_PROJECT_ID is required (game_config)")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
