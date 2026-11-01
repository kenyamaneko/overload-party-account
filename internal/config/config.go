// Package config は account サービスの起動設定を管理する。
// 環境変数が未設定または読めない値のときは、既定値で補わず起動時にエラーにする。
// CLOUDSQL_CONNECTION_NAME は DATABASE_IAM_AUTH_ENABLED が true のときだけ必須で、false のときは無視する。
package config

import (
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

const (
	minPort = 1
	maxPort = 65535
)

// LogMode はログの出力形式である。
type LogMode string

const (
	// LogModeProduction は Cloud Logging 互換の JSON で出力する。
	LogModeProduction LogMode = "production"
	// LogModeLocal は人間が読むテキストで出力する。
	LogModeLocal LogMode = "local"
)

// Config は account サービスの起動設定である。
type Config struct {
	Port int

	// DatabaseConn は PostgreSQL の接続文字列である。
	DatabaseConn string

	// GoogleCloudProjectID は Firestore のゲーム設定を読む Google Cloud プロジェクトの ID である。
	GoogleCloudProjectID string

	// InternalAuthPublicKey は X-Internal-Auth の RS256 JWT を検証する公開鍵である。
	InternalAuthPublicKey *rsa.PublicKey

	LogMode LogMode

	// IsDatabaseIAMAuthEnabled は Cloud SQL への接続に IAM 認証を使うかを表す。
	IsDatabaseIAMAuthEnabled bool

	// CloudSQLConnectionName は Cloud SQL インスタンスの接続名 (project:region:instance) である。
	CloudSQLConnectionName string
}

// New は環境変数から起動設定を読み込む。
func New() (*Config, error) {
	cfg := &Config{
		DatabaseConn:         os.Getenv("DATABASE_CONN"),
		GoogleCloudProjectID: os.Getenv("GOOGLE_CLOUD_PROJECT_ID"),
		LogMode:              LogMode(os.Getenv("LOG_MODE")),
	}

	rawPort := os.Getenv("PORT")
	if rawPort == "" {
		return nil, fmt.Errorf("config: PORT is required")
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return nil, fmt.Errorf("config: PORT %q: %w", rawPort, err)
	}
	if port < minPort || port > maxPort {
		return nil, fmt.Errorf("config: PORT must be in %d-%d, got %d", minPort, maxPort, port)
	}
	cfg.Port = port

	if cfg.DatabaseConn == "" {
		return nil, fmt.Errorf("config: DATABASE_CONN is required")
	}
	if cfg.GoogleCloudProjectID == "" {
		return nil, fmt.Errorf("config: GOOGLE_CLOUD_PROJECT_ID is required (Firestore (game_config) で必要)")
	}
	rawInternalAuthPublicKey := os.Getenv("INTERNAL_AUTH_PUBLIC_KEY")
	if rawInternalAuthPublicKey == "" {
		return nil, fmt.Errorf("config: INTERNAL_AUTH_PUBLIC_KEY is required (gateway が発行する RS256 JWT の検証鍵)")
	}
	internalAuthPublicKey, err := internalauth.ParsePublicKeyPEM([]byte(rawInternalAuthPublicKey))
	if err != nil {
		return nil, fmt.Errorf("config: INTERNAL_AUTH_PUBLIC_KEY is invalid: %w", err)
	}
	cfg.InternalAuthPublicKey = internalAuthPublicKey

	switch cfg.LogMode {
	case LogModeProduction, LogModeLocal:
	default:
		return nil, fmt.Errorf("config: LOG_MODE must be %q or %q, got %q", LogModeProduction, LogModeLocal, cfg.LogMode)
	}

	rawIAMAuth := os.Getenv("DATABASE_IAM_AUTH_ENABLED")
	switch rawIAMAuth {
	case "true":
		cfg.IsDatabaseIAMAuthEnabled = true
	case "false":
		cfg.IsDatabaseIAMAuthEnabled = false
	default:
		return nil, fmt.Errorf("config: DATABASE_IAM_AUTH_ENABLED must be %q or %q, got %q", "true", "false", rawIAMAuth)
	}
	if cfg.IsDatabaseIAMAuthEnabled {
		cfg.CloudSQLConnectionName = os.Getenv("CLOUDSQL_CONNECTION_NAME")
		if cfg.CloudSQLConnectionName == "" {
			return nil, fmt.Errorf("config: CLOUDSQL_CONNECTION_NAME is required when DATABASE_IAM_AUTH_ENABLED is true")
		}
	}

	return cfg, nil
}
