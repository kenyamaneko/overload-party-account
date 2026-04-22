// Package config は account サービスの起動設定を管理する。
//
// 全 env は必須。未設定・パース不能値は起動時に fail する
// （デフォルトへのフォールバックは CLAUDE.md の禁止事項）。
package config

import (
	"fmt"
	"os"
	"strconv"
)

// LogMode は log handler の選択を制御する。production は Cloud Logging 互換 JSON、
// local は人間向け TextHandler を使う。
type LogMode string

const (
	// LogModeProduction は本番環境で Cloud Logging 互換 JSON を使う。
	LogModeProduction LogMode = "production"
	// LogModeLocal はローカル開発で人間向け TextHandler を使う。
	LogModeLocal LogMode = "local"
)

// Config は account サービスの起動設定を保持する。
type Config struct {
	Port int

	// DatabaseURL は PostgreSQL 接続文字列（pgx が解釈可能な URL 形式 or libpq 形式）。
	DatabaseURL string

	// PubsubProjectID は account が subscribe する Pub/Sub topic を保有する Google Cloud project ID。
	PubsubProjectID string
	// FactionPurchasedSubscription は faction-purchased の pull subscription 名。
	FactionPurchasedSubscription string
	// PremiumUpdatedSubscription は premium-updated の pull subscription 名。
	PremiumUpdatedSubscription string
	// PlayerOnboardedSubscription は player-onboarded の pull subscription 名。
	PlayerOnboardedSubscription string

	// FirestoreProjectID は game_config の読み取り先プロジェクト ID。
	// ローカル/CI では FIRESTORE_EMULATOR_HOST を別途設定することでエミュレーターに接続する。
	FirestoreProjectID string

	// LogMode は log handler の選択。production / local のいずれか必須。
	LogMode LogMode
}

// FromEnv は環境変数から Config を構築する。
// 未設定・未定義値は起動時に fail する（デフォルトへのフォールバック禁止）。
func FromEnv() (*Config, error) {
	cfg := &Config{
		DatabaseURL:                  os.Getenv("DATABASE_URL"),
		PubsubProjectID:              os.Getenv("PUBSUB_PROJECT_ID"),
		FactionPurchasedSubscription: os.Getenv("FACTION_PURCHASED_SUBSCRIPTION"),
		PremiumUpdatedSubscription:   os.Getenv("PREMIUM_UPDATED_SUBSCRIPTION"),
		PlayerOnboardedSubscription:  os.Getenv("PLAYER_ONBOARDED_SUBSCRIPTION"),
		FirestoreProjectID:           os.Getenv("FIRESTORE_PROJECT_ID"),
		LogMode:                      LogMode(os.Getenv("LOG_MODE")),
	}

	rawPort := os.Getenv("PORT")
	if rawPort == "" {
		return nil, fmt.Errorf("config: PORT is required")
	}
	n, err := strconv.Atoi(rawPort)
	if err != nil {
		return nil, fmt.Errorf("config: PORT %q: %w", rawPort, err)
	}
	if n < 1 || n > 65535 {
		return nil, fmt.Errorf("config: PORT must be in 1-65535, got %d", n)
	}
	cfg.Port = n

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.PubsubProjectID == "" {
		return nil, fmt.Errorf("config: PUBSUB_PROJECT_ID is required (account subscribes to faction-purchased / premium-updated / player-onboarded)")
	}
	if cfg.FactionPurchasedSubscription == "" {
		return nil, fmt.Errorf("config: FACTION_PURCHASED_SUBSCRIPTION is required")
	}
	if cfg.PremiumUpdatedSubscription == "" {
		return nil, fmt.Errorf("config: PREMIUM_UPDATED_SUBSCRIPTION is required")
	}
	if cfg.PlayerOnboardedSubscription == "" {
		return nil, fmt.Errorf("config: PLAYER_ONBOARDED_SUBSCRIPTION is required")
	}
	if cfg.FirestoreProjectID == "" {
		return nil, fmt.Errorf("config: FIRESTORE_PROJECT_ID is required (game_config)")
	}

	switch cfg.LogMode {
	case LogModeProduction, LogModeLocal:
	default:
		return nil, fmt.Errorf("config: LOG_MODE must be %q or %q, got %q", LogModeProduction, LogModeLocal, cfg.LogMode)
	}

	return cfg, nil
}
