package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allEnvKeys は FromEnv が読む全 env キー。各テストは毎回これらを明示値（または ""）
// で上書きし、シェル環境からの漏れで Given が非決定になるのを防ぐ。
// testPublicKeyPEM は config が値をそのまま保持することの確認にだけ使うダミー。
// 鍵としての妥当性は検証しないため、PEM の体裁だけ揃えている。
const testPublicKeyPEM = "-----BEGIN PUBLIC KEY-----\ndummy-not-a-real-key\n-----END PUBLIC KEY-----\n"

var allEnvKeys = []string{
	"PORT",
	"DATABASE_CONN",
	"GOOGLE_CLOUD_PROJECT_ID",
	"INTERNAL_AUTH_PUBLIC_KEY",
	"LOG_MODE",
	"DATABASE_IAM_AUTH_ENABLED",
	"CLOUDSQL_CONNECTION_NAME",
}

// setEnv は allEnvKeys を一括で上書きする。envs に無いキーは "" として t.Setenv で
// 適用する — os.Getenv は "" と unset を区別しないため、空文字指定で required チェックの
// missing 経路を発火できる。t.Setenv はテスト終了時に復元する。
func setEnv(t *testing.T, envs map[string]string) {
	t.Helper()
	for _, k := range allEnvKeys {
		t.Setenv(k, envs[k])
	}
}

func mergeEnv(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// validLocalEnv は LOG_MODE=local での最小構成（必須 env を全て明示）。
// CLAUDE.md「デフォルト値へのフォールバックを行わない」方針により、全必須 env を
// 明示的に供給する。各ケースはこれを baseline に override を重ねる。
var validLocalEnv = map[string]string{
	"PORT":                      "9005",
	"DATABASE_CONN":             "host=localhost port=5432 dbname=account user=account password=account sslmode=disable",
	"GOOGLE_CLOUD_PROJECT_ID":   "account-local",
	"INTERNAL_AUTH_PUBLIC_KEY":  testPublicKeyPEM,
	"LOG_MODE":                  "local",
	"DATABASE_IAM_AUTH_ENABLED": "false",
}

func TestFromEnv(t *testing.T) {
	t.Run("環境変数からのConfig構築", func(t *testing.T) {
		t.Run("必須envが揃うとき、全フィールドがConfigに伝搬する", func(t *testing.T) {
			setEnv(t, validLocalEnv)

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.Equal(t, 9005, cfg.Port)
			assert.Equal(t, "host=localhost port=5432 dbname=account user=account password=account sslmode=disable", cfg.DatabaseConn)
			assert.Equal(t, "account-local", cfg.GoogleCloudProjectID)
			assert.Equal(t, LogModeLocal, cfg.LogMode)
		})

		t.Run("LOG_MODE=productionのとき、LogModeProductionとして受理される", func(t *testing.T) {
			setEnv(t, mergeEnv(validLocalEnv, map[string]string{"LOG_MODE": "production"}))

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.Equal(t, LogModeProduction, cfg.LogMode)
		})

		t.Run("PORTが指定されるとき、その値がConfigに反映される", func(t *testing.T) {
			setEnv(t, mergeEnv(validLocalEnv, map[string]string{"PORT": "8080"}))

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.Equal(t, 8080, cfg.Port)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDがfalseのとき、CLOUDSQL_CONNECTION_NAMEが未設定でも成功する", func(t *testing.T) {
			setEnv(t, validLocalEnv)

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.False(t, cfg.DatabaseIAMAuthEnabled)
			assert.Empty(t, cfg.CloudSQLConnectionName)
		})

		t.Run("DATABASE_IAM_AUTH_ENABLEDがtrueかつCLOUDSQL_CONNECTION_NAMEが指定されるとき、両方の値がConfigに反映される", func(t *testing.T) {
			setEnv(t, mergeEnv(validLocalEnv, map[string]string{
				"DATABASE_IAM_AUTH_ENABLED": "true",
				"CLOUDSQL_CONNECTION_NAME":  "overload-party-dev:asia-northeast1:overload-party-db",
			}))

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.True(t, cfg.DatabaseIAMAuthEnabled)
			assert.Equal(t, "overload-party-dev:asia-northeast1:overload-party-db", cfg.CloudSQLConnectionName)
		})

		// 必須 env が未設定・未定義値のときはデフォルトにフォールバックせず即エラーにする (回帰防止)。
		invalidCases := []struct {
			name    string
			envs    map[string]string
			wantErr string
		}{
			{
				name:    "PORTが未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": ""}),
				wantErr: "PORT is required",
			},
			{
				name:    "PORTが数値でないとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": "not-a-number"}),
				wantErr: "PORT",
			},
			{
				name:    "PORTが0のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": "0"}),
				wantErr: "PORT must be in 1-65535",
			},
			{
				name:    "PORTが65535超のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": "70000"}),
				wantErr: "PORT must be in 1-65535",
			},
			{
				name:    "DATABASE_CONNが未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"DATABASE_CONN": ""}),
				wantErr: "DATABASE_CONN is required",
			},
			{
				name:    "GOOGLE_CLOUD_PROJECT_IDが未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"GOOGLE_CLOUD_PROJECT_ID": ""}),
				wantErr: "GOOGLE_CLOUD_PROJECT_ID is required",
			},
			{
				name:    "INTERNAL_AUTH_PUBLIC_KEYが未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"INTERNAL_AUTH_PUBLIC_KEY": ""}),
				wantErr: "INTERNAL_AUTH_PUBLIC_KEY is required",
			},
			{
				name:    "LOG_MODEが未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"LOG_MODE": ""}),
				wantErr: "LOG_MODE must be",
			},
			{
				name:    "LOG_MODEが未定義値のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"LOG_MODE": "verbose"}),
				wantErr: "LOG_MODE must be",
			},
			{
				name:    "DATABASE_IAM_AUTH_ENABLEDが未設定のとき、変数名を含むエラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"DATABASE_IAM_AUTH_ENABLED": ""}),
				wantErr: "DATABASE_IAM_AUTH_ENABLED must be",
			},
			{
				name:    `DATABASE_IAM_AUTH_ENABLEDが "true"/"false" 以外の "yes" のとき、変数名を含むエラーになる`,
				envs:    mergeEnv(validLocalEnv, map[string]string{"DATABASE_IAM_AUTH_ENABLED": "yes"}),
				wantErr: "DATABASE_IAM_AUTH_ENABLED must be",
			},
			{
				name: "DATABASE_IAM_AUTH_ENABLEDがtrueかつCLOUDSQL_CONNECTION_NAMEが未設定のとき、CLOUDSQL_CONNECTION_NAMEを含むエラーになる",
				envs: mergeEnv(validLocalEnv, map[string]string{
					"DATABASE_IAM_AUTH_ENABLED": "true",
					"CLOUDSQL_CONNECTION_NAME":  "",
				}),
				wantErr: "CLOUDSQL_CONNECTION_NAME is required",
			},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				setEnv(t, tc.envs)

				_, err := FromEnv()

				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			})
		}
	})
}
