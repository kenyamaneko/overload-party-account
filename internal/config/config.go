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

	// GoogleCloudProjectID は account が利用する Google Cloud 系サービス
	// (Firestore game_config 読み取り) の project ID。
	// ローカル/CI では FIRESTORE_EMULATOR_HOST 経由でエミュレーターに接続する。
	GoogleCloudProjectID string

	// InternalAuthSecret は gateway / 各サービス間で共有する HMAC 鍵。
	// X-Internal-Auth (HS256 JWT) の検証に使う。ADR-037 参照。
	InternalAuthSecret string

	// LogMode は log handler の選択。production / local のいずれか必須。
	LogMode LogMode
}

// FromEnv は環境変数から Config を構築する。
func FromEnv() (*Config, error) {
	cfg := &Config{
		DatabaseConn:         os.Getenv("DATABASE_CONN"),
		GoogleCloudProjectID: os.Getenv("GOOGLE_CLOUD_PROJECT_ID"),
		InternalAuthSecret:   os.Getenv("INTERNAL_AUTH_SECRET"),
		LogMode:              LogMode(os.Getenv("LOG_MODE")),
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
	if cfg.GoogleCloudProjectID == "" {
		return nil, fmt.Errorf("config: GOOGLE_CLOUD_PROJECT_ID is required (Firestore (game_config) で必要)")
	}
	if cfg.InternalAuthSecret == "" {
		return nil, fmt.Errorf("config: INTERNAL_AUTH_SECRET is required (HS256 JWT shared secret, see ADR-037)")
	}

	switch cfg.LogMode {
	case LogModeProduction, LogModeLocal:
	default:
		return nil, fmt.Errorf("config: LOG_MODE must be %q or %q, got %q", LogModeProduction, LogModeLocal, cfg.LogMode)
	}

	return cfg, nil
}
