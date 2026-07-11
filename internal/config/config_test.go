package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allEnvKeys は FromEnv が読む全 env キー。各テストは毎回これらを明示値（または ""）
// で上書きし、シェル環境からの漏れで Given が非決定になるのを防ぐ。
var allEnvKeys = []string{
	"PORT",
	"DATABASE_CONN",
	"GOOGLE_CLOUD_PROJECT_ID",
	"FACTION_ACQUIRED_SUBSCRIPTION",
	"PREMIUM_UPDATED_SUBSCRIPTION",
	"PLAYER_ONBOARDED_SUBSCRIPTION",
	"ONBOARDING_NAME_SET_SUBSCRIPTION",
	"ONBOARDING_FACTION_SET_SUBSCRIPTION",
	"INTERNAL_AUTH_SECRET",
	"LOG_MODE",
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
	"PORT":                                "9005",
	"DATABASE_CONN":                       "host=localhost port=5432 dbname=account user=account password=account sslmode=disable",
	"GOOGLE_CLOUD_PROJECT_ID":             "account-local",
	"FACTION_ACQUIRED_SUBSCRIPTION":       "faction-acquired-account-sub",
	"PREMIUM_UPDATED_SUBSCRIPTION":        "premium-updated-account-sub",
	"PLAYER_ONBOARDED_SUBSCRIPTION":       "player-onboarded-account-sub",
	"ONBOARDING_NAME_SET_SUBSCRIPTION":    "onboarding-name-set-account-sub",
	"ONBOARDING_FACTION_SET_SUBSCRIPTION": "onboarding-faction-set-account-sub",
	"INTERNAL_AUTH_SECRET":                "test-internal-auth-secret-do-not-use-in-prod-xxxxx",
	"LOG_MODE":                            "local",
}

func TestFromEnv(t *testing.T) {
	t.Run("環境変数からの Config 構築", func(t *testing.T) {
		t.Run("必須 env が揃うとき、全フィールドが Config に伝搬する", func(t *testing.T) {
			setEnv(t, validLocalEnv)

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.Equal(t, 9005, cfg.Port)
			assert.Equal(t, "host=localhost port=5432 dbname=account user=account password=account sslmode=disable", cfg.DatabaseConn)
			assert.Equal(t, "account-local", cfg.GoogleCloudProjectID)
			assert.Equal(t, "faction-acquired-account-sub", cfg.FactionAcquiredSubscription)
			assert.Equal(t, "premium-updated-account-sub", cfg.PremiumUpdatedSubscription)
			assert.Equal(t, "player-onboarded-account-sub", cfg.PlayerOnboardedSubscription)
			assert.Equal(t, LogModeLocal, cfg.LogMode)
		})

		t.Run("LOG_MODE=production のとき、LogModeProduction として受理される", func(t *testing.T) {
			setEnv(t, mergeEnv(validLocalEnv, map[string]string{"LOG_MODE": "production"}))

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.Equal(t, LogModeProduction, cfg.LogMode)
		})

		t.Run("PORT が指定されるとき、その値が Config に反映される", func(t *testing.T) {
			setEnv(t, mergeEnv(validLocalEnv, map[string]string{"PORT": "8080"}))

			cfg, err := FromEnv()

			require.NoError(t, err)
			assert.Equal(t, 8080, cfg.Port)
		})

		// 必須 env が未設定・未定義値のときはデフォルトにフォールバックせず即エラーにする (回帰防止)。
		invalidCases := []struct {
			name    string
			envs    map[string]string
			wantErr string
		}{
			{
				name:    "PORT が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": ""}),
				wantErr: "PORT is required",
			},
			{
				name:    "PORT が数値でないとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": "not-a-number"}),
				wantErr: "PORT",
			},
			{
				name:    "PORT が 0 のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": "0"}),
				wantErr: "PORT must be in 1-65535",
			},
			{
				name:    "PORT が 65535 超のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PORT": "70000"}),
				wantErr: "PORT must be in 1-65535",
			},
			{
				name:    "DATABASE_CONN が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"DATABASE_CONN": ""}),
				wantErr: "DATABASE_CONN is required",
			},
			{
				name:    "GOOGLE_CLOUD_PROJECT_ID が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"GOOGLE_CLOUD_PROJECT_ID": ""}),
				wantErr: "GOOGLE_CLOUD_PROJECT_ID is required",
			},
			{
				name:    "FACTION_ACQUIRED_SUBSCRIPTION が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"FACTION_ACQUIRED_SUBSCRIPTION": ""}),
				wantErr: "FACTION_ACQUIRED_SUBSCRIPTION is required",
			},
			{
				name:    "PREMIUM_UPDATED_SUBSCRIPTION が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PREMIUM_UPDATED_SUBSCRIPTION": ""}),
				wantErr: "PREMIUM_UPDATED_SUBSCRIPTION is required",
			},
			{
				name:    "PLAYER_ONBOARDED_SUBSCRIPTION が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"PLAYER_ONBOARDED_SUBSCRIPTION": ""}),
				wantErr: "PLAYER_ONBOARDED_SUBSCRIPTION is required",
			},
			{
				name:    "ONBOARDING_NAME_SET_SUBSCRIPTION が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"ONBOARDING_NAME_SET_SUBSCRIPTION": ""}),
				wantErr: "ONBOARDING_NAME_SET_SUBSCRIPTION is required",
			},
			{
				name:    "ONBOARDING_FACTION_SET_SUBSCRIPTION が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"ONBOARDING_FACTION_SET_SUBSCRIPTION": ""}),
				wantErr: "ONBOARDING_FACTION_SET_SUBSCRIPTION is required",
			},
			{
				name:    "INTERNAL_AUTH_SECRET が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"INTERNAL_AUTH_SECRET": ""}),
				wantErr: "INTERNAL_AUTH_SECRET is required",
			},
			{
				name:    "LOG_MODE が未設定のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"LOG_MODE": ""}),
				wantErr: "LOG_MODE must be",
			},
			{
				name:    "LOG_MODE が未定義値のとき、エラーになる",
				envs:    mergeEnv(validLocalEnv, map[string]string{"LOG_MODE": "verbose"}),
				wantErr: "LOG_MODE must be",
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
