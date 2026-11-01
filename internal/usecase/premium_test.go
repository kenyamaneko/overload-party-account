package usecase_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

var (
	reflectedPremiumOccurredAt = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	reflectedPremiumExpiresAt  = time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
)

func newEnvWithReflectedPremiumEvent(t *testing.T) (*usecasetest.UnitEnv, string) {
	t.Helper()
	env := usecasetest.NewUnitEnv(t)
	playerID := usecasetest.RegisterPlayer(t, env.Auth)
	expiresAt := reflectedPremiumExpiresAt
	require.NoError(t, env.Premium.ApplyPremiumUpdated(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, true, &expiresAt, reflectedPremiumOccurredAt))
	return env, playerID
}

func applyPremiumRevocationEvent(t *testing.T, env *usecasetest.UnitEnv, playerID string, occurredAt time.Time) {
	t.Helper()
	require.NoError(t, env.Premium.ApplyPremiumUpdated(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, false, nil, occurredAt))
}

func TestApplyPremiumUpdated(t *testing.T) {
	t.Run("[プレミアム状態]更新イベントの反映", func(t *testing.T) {
		t.Run("is_premiumがtrue・premium_expires_atが2031-01-01T00:00:00Z・発生時刻が2030-01-01T00:00:00Zのイベントを反映済みのプレイヤーに、別のevent_idで、is_premiumがfalseでpremium_expires_atが無いイベントを反映するとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("発生時刻が2030-01-01T00:00:00Zで反映済みのイベントと同じとき", func(t *testing.T) {
					t.Run("反映したあとに取得したプレイヤー情報のis_premiumはfalseになる", func(t *testing.T) {
						env, playerID := newEnvWithReflectedPremiumEvent(t)
						applyPremiumRevocationEvent(t, env, playerID, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
						assert.False(t, playerInfoOf(t, env, playerID).View.Player.IsPremium)
					})

					t.Run("反映したあとに取得したプレイヤー情報にpremium_expires_atは含まれない", func(t *testing.T) {
						env, playerID := newEnvWithReflectedPremiumEvent(t)
						applyPremiumRevocationEvent(t, env, playerID, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
						assert.Nil(t, playerInfoOf(t, env, playerID).View.Player.PremiumExpiresAt)
					})
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("発生時刻が2029-12-31T23:59:59.999999999Zで反映済みのイベントより古いとき", func(t *testing.T) {
					t.Run("反映したあとに取得したプレイヤー情報のis_premiumはtrueのままになる", func(t *testing.T) {
						env, playerID := newEnvWithReflectedPremiumEvent(t)
						applyPremiumRevocationEvent(t, env, playerID, time.Date(2029, 12, 31, 23, 59, 59, 999999999, time.UTC))
						assert.True(t, playerInfoOf(t, env, playerID).View.Player.IsPremium)
					})

					t.Run("反映したあとに取得したプレイヤー情報のpremium_expires_atは2031-01-01T00:00:00Zのままになる", func(t *testing.T) {
						env, playerID := newEnvWithReflectedPremiumEvent(t)
						applyPremiumRevocationEvent(t, env, playerID, time.Date(2029, 12, 31, 23, 59, 59, 999999999, time.UTC))
						expiresAt := playerInfoOf(t, env, playerID).View.Player.PremiumExpiresAt
						require.NotNil(t, expiresAt)
						assert.Equal(t, time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC), expiresAt.UTC())
					})
				})
			})
		})

		t.Run("正常系", func(t *testing.T) {
			t.Run("まだイベントを反映していない非プレミアムのプレイヤーに、is_premiumがtrueのイベントを反映すると、反映したあとに取得したプレイヤー情報のis_premiumはtrueになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				require.NoError(t, env.Premium.ApplyPremiumUpdated(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, true, nil, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)))
				assert.True(t, playerInfoOf(t, env, playerID).View.Player.IsPremium)
			})
		})
	})
}
