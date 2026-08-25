package config_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/config"
)

// validPublicKeyPEM は起動時設定検証に使う、テスト専用に生成した RSA 公開鍵の PEM 文字列を返す。
func validPublicKeyPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return string(pem.EncodeToMemory(block))
}

// setValidRequiredEnv は FromEnv が要求する必須環境変数を全て有効な値で設定する。
// 各テストケースは、検証対象の 1 項目だけを上書きすることで、その項目だけを独立して検証できる。
func setValidRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("PORT", "9005")
	t.Setenv("DATABASE_CONN", "postgres://account:account@localhost:5432/account?sslmode=disable")
	t.Setenv("GOOGLE_CLOUD_PROJECT_ID", "overload-party-local")
	t.Setenv("INTERNAL_AUTH_PUBLIC_KEY", validPublicKeyPEM(t))
	t.Setenv("LOG_MODE", "local")
	t.Setenv("DATABASE_IAM_AUTH_ENABLED", "false")
}

func TestFromEnv(t *testing.T) {
	t.Run("起動設定の構築", func(t *testing.T) {
		t.Run("PORTが未設定のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("PORT", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("PORTが整数として解釈できないとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("PORT", "not-a-number")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("PORTが0のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("PORT", "0")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("PORTが1(下限)のとき、エラーにならない", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("PORT", "1")

			_, err := config.FromEnv()

			require.NoError(t, err)
		})

		t.Run("PORTが65535(上限)のとき、エラーにならない", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("PORT", "65535")

			_, err := config.FromEnv()

			require.NoError(t, err)
		})

		t.Run("PORTが65536のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("PORT", "65536")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("DATABASE_CONNが未設定(空文字)のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("DATABASE_CONN", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("GOOGLE_CLOUD_PROJECT_IDが未設定のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("GOOGLE_CLOUD_PROJECT_ID", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("INTERNAL_AUTH_PUBLIC_KEYが未設定のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("INTERNAL_AUTH_PUBLIC_KEY", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("LOG_MODEがproductionとlocalのいずれでもないとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("LOG_MODE", "bogus")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("LOG_MODEが未設定のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("LOG_MODE", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("LOG_MODEがproductionのとき、エラーにならない", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("LOG_MODE", "production")

			_, err := config.FromEnv()

			require.NoError(t, err)
		})

		t.Run("LOG_MODEがlocalのとき、エラーにならない", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("LOG_MODE", "local")

			_, err := config.FromEnv()

			require.NoError(t, err)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDがtrueとfalseのいずれの文字列でもないとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("DATABASE_IAM_AUTH_ENABLED", "bogus")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDが未設定のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("DATABASE_IAM_AUTH_ENABLED", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDがtrueでCLOUDSQL_CONNECTION_NAMEが未設定のとき、エラーを返す", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("DATABASE_IAM_AUTH_ENABLED", "true")
			t.Setenv("CLOUDSQL_CONNECTION_NAME", "")

			_, err := config.FromEnv()

			require.Error(t, err)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDがtrueでCLOUDSQL_CONNECTION_NAMEが設定されているとき、エラーにならない", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("DATABASE_IAM_AUTH_ENABLED", "true")
			t.Setenv("CLOUDSQL_CONNECTION_NAME", "proj:region:instance")

			_, err := config.FromEnv()

			require.NoError(t, err)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDがfalseのとき、CLOUDSQL_CONNECTION_NAMEが未設定でもエラーにならない", func(t *testing.T) {
			setValidRequiredEnv(t)
			t.Setenv("DATABASE_IAM_AUTH_ENABLED", "false")
			t.Setenv("CLOUDSQL_CONNECTION_NAME", "")

			_, err := config.FromEnv()

			require.NoError(t, err)
		})

		t.Run("全ての必須環境変数が有効な値で設定されているとき、Configの各フィールドに対応する値がそのまま読み込まれる", func(t *testing.T) {
			setValidRequiredEnv(t)
			pemKey := validPublicKeyPEM(t)
			t.Setenv("INTERNAL_AUTH_PUBLIC_KEY", pemKey)
			t.Setenv("PORT", "9005")
			t.Setenv("DATABASE_CONN", "postgres://account:account@localhost:5432/account?sslmode=disable")
			t.Setenv("GOOGLE_CLOUD_PROJECT_ID", "overload-party-local")
			t.Setenv("LOG_MODE", "production")
			t.Setenv("DATABASE_IAM_AUTH_ENABLED", "true")
			t.Setenv("CLOUDSQL_CONNECTION_NAME", "proj:region:instance")

			cfg, err := config.FromEnv()

			require.NoError(t, err)
			assert.Equal(t, 9005, cfg.Port)
			assert.Equal(t, "postgres://account:account@localhost:5432/account?sslmode=disable", cfg.DatabaseConn)
			assert.Equal(t, "overload-party-local", cfg.GoogleCloudProjectID)
			assert.Equal(t, pemKey, cfg.InternalAuthPublicKey)
			assert.Equal(t, config.LogModeProduction, cfg.LogMode)
			assert.Equal(t, true, cfg.DatabaseIAMAuthEnabled)
			assert.Equal(t, "proj:region:instance", cfg.CloudSQLConnectionName)
		})
	})
}
