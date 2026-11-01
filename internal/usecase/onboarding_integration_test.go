//go:build integration

package usecase_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func playerInfoFromIntegration(t *testing.T, env *usecasetest.IntegrationEnv, playerID string) *domain.PlayerInfo {
	t.Helper()
	info, err := env.Player.GetPlayer(t.Context(), playerID)
	require.NoError(t, err)
	return info
}

func newEnvWithFactionConflict(t *testing.T) (env *usecasetest.IntegrationEnv, playerID, failedEventID string) {
	t.Helper()
	env = usecasetest.NewIntegrationEnv(t, sharedPg)
	playerID = usecasetest.RegisterPlayer(t, env.Auth)
	usecasetest.AdvanceOnboarding(t, env.Onboarding, playerID, usecasetest.StepsToFactionSet)
	failedEventID = uuid.NewString()
	err := env.Onboarding.ApplyFactionSet(t.Context(), failedEventID, usecasetest.DummyEventType, playerID, "Tenki")
	require.ErrorIs(t, err, usecase.ErrFactionConflict)
	return env, playerID, failedEventID
}

func TestOnboardingEventApplication(t *testing.T) {
	t.Run("[オンボーディング進行]イベントの反映", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーのイベントを反映するとき", func(t *testing.T) {
				tests := []struct {
					name  string
					apply func(env *usecasetest.IntegrationEnv, playerID string) error
				}{
					{"表示名設定イベントのとき、プレイヤーが存在しないことを示すエラーになる", func(env *usecasetest.IntegrationEnv, playerID string) error {
						return env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ")
					}},
					{"陣営設定イベントのとき、プレイヤーが存在しないことを示すエラーになる", func(env *usecasetest.IntegrationEnv, playerID string) error {
						return env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE")
					}},
					{"完了イベントのとき、プレイヤーが存在しないことを示すエラーになる", func(env *usecasetest.IntegrationEnv, playerID string) error {
						return env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID)
					}},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						env := usecasetest.NewIntegrationEnv(t, sharedPg)
						err := tt.apply(env, uuid.NewString())
						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})

			t.Run("未登録のプレイヤーへの表示名設定イベントの反映に失敗したあと、同じevent_idの表示名設定イベントを登録済みのプレイヤーに反映し直すと、取得したプレイヤー情報のonboarding_statusはname_setになる", func(t *testing.T) {
				env := usecasetest.NewIntegrationEnv(t, sharedPg)
				eventID := uuid.NewString()
				err := env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, uuid.NewString(), "ゆうしゃ")
				require.ErrorIs(t, err, port.ErrNotFound)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "ゆうしゃ"))
				assert.Equal(t, "name_set", playerInfoFromIntegration(t, env, playerID).View.Player.OnboardingStatus)
			})

			t.Run("初期陣営にSHEが確定済みのプレイヤーへのTenkiの陣営設定イベントが衝突で失敗したあと", func(t *testing.T) {
				t.Run("同じevent_idのイベントを反映し直すと、再び陣営の衝突を示すエラーになる", func(t *testing.T) {
					env, playerID, failedEventID := newEnvWithFactionConflict(t)
					err := env.Onboarding.ApplyFactionSet(t.Context(), failedEventID, usecasetest.DummyEventType, playerID, "Tenki")
					assert.ErrorIs(t, err, usecase.ErrFactionConflict)
				})

				t.Run("プレイヤー情報を取得すると、initial_factionはSHEのままになる", func(t *testing.T) {
					env, playerID, _ := newEnvWithFactionConflict(t)
					assert.Equal(t, usecasetest.ToPointer("SHE"), playerInfoFromIntegration(t, env, playerID).View.InitialFaction)
				})

				t.Run("完了イベントを反映すると、取得したプレイヤー情報のonboarding_statusはcompletedになる", func(t *testing.T) {
					env, playerID, _ := newEnvWithFactionConflict(t)
					require.NoError(t, env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID))
					assert.Equal(t, "completed", playerInfoFromIntegration(t, env, playerID).View.Player.OnboardingStatus)
				})
			})

			t.Run("onboarding_statusがnot_startedのプレイヤーへの陣営設定イベントが段階を飛ばしていて失敗したあと", func(t *testing.T) {
				t.Run("プレイヤー情報を取得すると、initial_factionは含まれない", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE")
					require.ErrorIs(t, err, domain.ErrOnboardingStatusSkipped)
					assert.Nil(t, playerInfoFromIntegration(t, env, playerID).View.InitialFaction)
				})

				t.Run("プレイヤー情報を取得すると、onboarding_statusはnot_startedのままになる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE")
					require.ErrorIs(t, err, domain.ErrOnboardingStatusSkipped)
					assert.Equal(t, "not_started", playerInfoFromIntegration(t, env, playerID).View.Player.OnboardingStatus)
				})

				t.Run("表示名設定イベントを反映してから、先に失敗した同じevent_idの陣営がSHEの陣営設定イベントを反映し直すと、取得したプレイヤー情報のinitial_factionはSHEになる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					failedEventID := uuid.NewString()
					err := env.Onboarding.ApplyFactionSet(t.Context(), failedEventID, usecasetest.DummyEventType, playerID, "SHE")
					require.ErrorIs(t, err, domain.ErrOnboardingStatusSkipped)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ"))
					require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), failedEventID, usecasetest.DummyEventType, playerID, "SHE"))
					assert.Equal(t, usecasetest.ToPointer("SHE"), playerInfoFromIntegration(t, env, playerID).View.InitialFaction)
				})
			})
		})
	})
}
