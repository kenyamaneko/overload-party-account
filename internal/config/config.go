// Package config は account サービスの起動設定を管理する。
// 全 env は必須で、未設定・パース不能値は起動時に fail する (CLAUDE.md: デフォルトへのフォールバック禁止)。
package config

import (
	"fmt"
	"os"
	"strconv"
)

// LogMode は log handler の選択を制御する。
type LogMode string

const (
	// LogModeProduction は Cloud Logging 互換 JSON を使う。
	LogModeProduction LogMode = "production"
	// LogModeLocal は人間向け TextHandler を使う。
	LogModeLocal LogMode = "local"
)

// Config は account サービスの起動設定を保持する。
type Config struct {
	Port int

	// DatabaseConn は PostgreSQL 接続文字列（libpq キーワード形式）。
	DatabaseConn string

	// PubsubProjectID は account が subscribe する Pub/Sub topic を保有する Google Cloud project ID。
	PubsubProjectID string
	// FactionAcquiredSubscription は faction-acquired の pull subscription 名。
	FactionAcquiredSubscription string
	// PremiumUpdatedSubscription は premium-updated の pull subscription 名。
	PremiumUpdatedSubscription string
	// PlayerOnboardedSubscription は player-onboarded の pull subscription 名。
	PlayerOnboardedSubscription string
	// OnboardingNameSetSubscription は onboarding-name-set の pull subscription 名。
	OnboardingNameSetSubscription string
	// OnboardingFactionSetSubscription は onboarding-faction-set の pull subscription 名。
	OnboardingFactionSetSubscription string

	// FirestoreProjectID は game_config の読み取り先プロジェクト ID。
	// ローカル/CI では FIRESTORE_EMULATOR_HOST 経由でエミュレーターに接続する。
	FirestoreProjectID string

	// LogMode は log handler の選択。production / local のいずれか必須。
	LogMode LogMode
}

// FromEnv は環境変数から Config を構築する。
func FromEnv() (*Config, error) {
	cfg := &Config{
		DatabaseConn:                     os.Getenv("DATABASE_CONN"),
		PubsubProjectID:                  os.Getenv("PUBSUB_PROJECT_ID"),
		FactionAcquiredSubscription:      os.Getenv("FACTION_ACQUIRED_SUBSCRIPTION"),
		PremiumUpdatedSubscription:       os.Getenv("PREMIUM_UPDATED_SUBSCRIPTION"),
		PlayerOnboardedSubscription:      os.Getenv("PLAYER_ONBOARDED_SUBSCRIPTION"),
		OnboardingNameSetSubscription:    os.Getenv("ONBOARDING_NAME_SET_SUBSCRIPTION"),
		OnboardingFactionSetSubscription: os.Getenv("ONBOARDING_FACTION_SET_SUBSCRIPTION"),
		FirestoreProjectID:               os.Getenv("FIRESTORE_PROJECT_ID"),
		LogMode:                          LogMode(os.Getenv("LOG_MODE")),
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

	if cfg.DatabaseConn == "" {
		return nil, fmt.Errorf("config: DATABASE_CONN is required")
	}
	if cfg.PubsubProjectID == "" {
		return nil, fmt.Errorf("config: PUBSUB_PROJECT_ID is required (account subscribes to faction-acquired / premium-updated / player-onboarded)")
	}
	if cfg.FactionAcquiredSubscription == "" {
		return nil, fmt.Errorf("config: FACTION_ACQUIRED_SUBSCRIPTION is required")
	}
	if cfg.PremiumUpdatedSubscription == "" {
		return nil, fmt.Errorf("config: PREMIUM_UPDATED_SUBSCRIPTION is required")
	}
	if cfg.PlayerOnboardedSubscription == "" {
		return nil, fmt.Errorf("config: PLAYER_ONBOARDED_SUBSCRIPTION is required")
	}
	if cfg.OnboardingNameSetSubscription == "" {
		return nil, fmt.Errorf("config: ONBOARDING_NAME_SET_SUBSCRIPTION is required")
	}
	if cfg.OnboardingFactionSetSubscription == "" {
		return nil, fmt.Errorf("config: ONBOARDING_FACTION_SET_SUBSCRIPTION is required")
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
