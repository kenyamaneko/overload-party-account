package usecase_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

const onboardingPlayerID = "d4b1c7e9-2a5f-4e83-9c06-7f1a3b5d8e20"

func newOnboardingEnv(t *testing.T, stepCount int) (*usecasetest.UnitEnv, string) {
	t.Helper()
	env := usecasetest.NewUnitEnvWithPlayerIDGenerator(t, porttest.NewFixedPlayerIDGenerator(onboardingPlayerID))
	playerID := usecasetest.RegisterPlayer(t, env.Auth)
	usecasetest.AdvanceOnboarding(t, env.Onboarding, playerID, stepCount)
	return env, playerID
}

func playerInfoOf(t *testing.T, env *usecasetest.UnitEnv, playerID string) *domain.PlayerInfo {
	t.Helper()
	info, err := env.Player.GetPlayer(t.Context(), playerID)
	require.NoError(t, err)
	return info
}

func TestApplyNameSet(t *testing.T) {
	t.Run("[オンボーディング進行]表示名設定イベントの反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録直後のプレイヤーに未処理のイベントを反映するとき", func(t *testing.T) {
				t.Run("表示名がゆうしゃのとき、反映したあとに取得したプレイヤー情報のnameはゆうしゃになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					err := env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ")
					require.NoError(t, err)
					assert.Equal(t, usecasetest.ToPointer("ゆうしゃ"), playerInfoOf(t, env, playerID).View.Player.Name)
				})

				t.Run("反映したあとに取得したプレイヤー情報のonboarding_statusはname_setになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					err := env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ")
					require.NoError(t, err)
					assert.Equal(t, "name_set", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
				})
			})

			t.Run("同じevent_idのイベントを2回反映するとき", func(t *testing.T) {
				t.Run("2回目の反映は、エラーにならない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					eventID := uuid.NewString()
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "ゆうしゃ"))
					assert.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "ゆうしゃ"))
				})

				t.Run("2回目の表示名がゆうしゃ2のとき、2回目を反映しても、反映したあとに取得したプレイヤー情報のnameはゆうしゃのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					eventID := uuid.NewString()
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "ゆうしゃ"))
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Equal(t, usecasetest.ToPointer("ゆうしゃ"), playerInfoOf(t, env, playerID).View.Player.Name)
				})
			})

			t.Run("onboarding_statusがname_setで表示名がゆうしゃのプレイヤーに、別のevent_idで表示名ゆうしゃ2のイベントを反映するとき、反映したあとに取得したプレイヤー情報のnameはゆうしゃ2になる", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
				require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
				assert.Equal(t, usecasetest.ToPointer("ゆうしゃ2"), playerInfoOf(t, env, playerID).View.Player.Name)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("空文字の表示名のイベントを反映するとき", func(t *testing.T) {
				t.Run("表示名が不正であることを示すエラーになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					err := env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "")
					assert.ErrorIs(t, err, domain.ErrInvalidName)
				})

				t.Run("反映に失敗したあと、同じevent_idで表示名ゆうしゃのイベントを反映すると、反映したあとに取得したプレイヤー情報のnameはゆうしゃになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					eventID := uuid.NewString()
					require.Error(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, ""))
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "ゆうしゃ"))
					assert.Equal(t, usecasetest.ToPointer("ゆうしゃ"), playerInfoOf(t, env, playerID).View.Player.Name)
				})

				t.Run("反映に失敗したあとにプレイヤー情報を取得すると、nameは含まれない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, 0)
					require.Error(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, ""))
					assert.Nil(t, playerInfoOf(t, env, playerID).View.Player.Name)
				})
			})
		})
	})
}

func TestApplyFactionSet(t *testing.T) {
	t.Run("[オンボーディング進行]陣営設定イベントの反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("onboarding_statusがname_setのプレイヤーに未処理のイベントを反映するとき", func(t *testing.T) {
				t.Run("陣営がSHEのとき、反映したあとに取得したプレイヤー情報のinitial_factionはSHEになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE"))
					assert.Equal(t, usecasetest.ToPointer("SHE"), playerInfoOf(t, env, playerID).View.InitialFaction)
				})

				t.Run("反映したあとに取得したプレイヤー情報のonboarding_statusはfaction_setになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE"))
					assert.Equal(t, "faction_set", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
				})
			})

			t.Run("onboarding_statusがname_setのプレイヤーに同じevent_idの陣営設定イベントを2回反映すると、2回目の反映は、エラーにならない", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
				eventID := uuid.NewString()
				require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "SHE"))
				assert.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), eventID, usecasetest.DummyEventType, playerID, "SHE"))
			})

			t.Run("初期陣営にSHEが確定済みのプレイヤーに、別のevent_idでSHEのイベントを反映するとき", func(t *testing.T) {
				t.Run("エラーにならない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
					assert.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE"))
				})

				t.Run("反映したあとに取得したプレイヤー情報のinitial_factionはSHEのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
					require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE"))
					assert.Equal(t, usecasetest.ToPointer("SHE"), playerInfoOf(t, env, playerID).View.InitialFaction)
				})

				t.Run("反映したあとに取得したプレイヤー情報のonboarding_statusはfaction_setのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
					require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE"))
					assert.Equal(t, "faction_set", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("初期陣営にSHEが確定済みのプレイヤーに、別のevent_idでTenkiのイベントを反映するとき", func(t *testing.T) {
				t.Run("陣営の衝突を示すエラーになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
					err := env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Tenki")
					assert.ErrorIs(t, err, usecase.ErrFactionConflict)
				})

				t.Run("反映に失敗したあとにプレイヤー情報を取得すると、initial_factionはSHEのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
					require.Error(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Tenki"))
					assert.Equal(t, usecasetest.ToPointer("SHE"), playerInfoOf(t, env, playerID).View.InitialFaction)
				})
			})

			t.Run("onboarding_statusがname_setのプレイヤーに、陣営がNeutralのイベントを反映するとき", func(t *testing.T) {
				t.Run("不正な陣営を示すエラーになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					err := env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Neutral")
					assert.ErrorIs(t, err, usecase.ErrInvalidFaction)
				})

				t.Run("反映に失敗したあとにプレイヤー情報を取得すると、initial_factionは含まれない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					require.Error(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Neutral"))
					assert.Nil(t, playerInfoOf(t, env, playerID).View.InitialFaction)
				})
			})

			t.Run("onboarding_statusがnot_startedのプレイヤーに陣営設定イベントを反映すると、段階を飛ばしていることを示すエラーになる", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, 0)
				err := env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE")
				assert.ErrorIs(t, err, domain.ErrOnboardingStatusSkipped)
			})
		})
	})
}

func TestApplyCompleted(t *testing.T) {
	t.Run("[オンボーディング進行]完了イベントの反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("onboarding_statusがfaction_setのプレイヤーに未処理のイベントを反映すると、反映したあとに取得したプレイヤー情報のonboarding_statusはcompletedになる", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
				require.NoError(t, env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID))
				assert.Equal(t, "completed", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
			})

			t.Run("onboarding_statusがfaction_setのプレイヤーに同じevent_idのイベントを2回反映すると、2回目の反映は、エラーにならない", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
				eventID := uuid.NewString()
				require.NoError(t, env.Onboarding.ApplyCompleted(t.Context(), eventID, usecasetest.DummyEventType, playerID))
				assert.NoError(t, env.Onboarding.ApplyCompleted(t.Context(), eventID, usecasetest.DummyEventType, playerID))
			})

			t.Run("onboarding_statusがcompletedのプレイヤーに別のevent_idのイベントを反映するとき", func(t *testing.T) {
				t.Run("エラーにならない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					assert.NoError(t, env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID))
				})

				t.Run("警告ログは出ない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					logs := loggingtest.Capture(t)
					require.NoError(t, env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID))
					assert.Empty(t, usecasetest.SelectWarnings(logs))
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("onboarding_statusがname_setのプレイヤーに未処理のイベントを反映するとき", func(t *testing.T) {
				t.Run("段階を飛ばしていることを示すエラーになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					err := env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID)
					assert.ErrorIs(t, err, domain.ErrOnboardingStatusSkipped)
				})

				t.Run("反映に失敗したあとにプレイヤー情報を取得すると、onboarding_statusはname_setのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					require.Error(t, env.Onboarding.ApplyCompleted(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID))
					assert.Equal(t, "name_set", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
				})
			})
		})
	})
}

func TestOnboardingStatusNotRegressing(t *testing.T) {
	t.Run("[オンボーディング進行]進行状態の後戻り防止", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("onboarding_statusがname_setのプレイヤーに表示名設定イベントを反映するとき", func(t *testing.T) {
				t.Run("反映したあとに取得したプレイヤー情報のonboarding_statusはname_setのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Equal(t, "name_set", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
				})

				t.Run("警告ログは出ない", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToNameSet)
					logs := loggingtest.Capture(t)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Empty(t, usecasetest.SelectWarnings(logs))
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("onboarding_statusがfaction_setのプレイヤーに表示名設定イベントを反映すると、反映したあとに取得したプレイヤー情報のonboarding_statusはfaction_setのままになる", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, usecasetest.StepsToFactionSet)
				require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
				assert.Equal(t, "faction_set", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
			})

			t.Run("onboarding_statusがcompletedで初期陣営がSHEのプレイヤーに、陣営がSHEの陣営設定イベントを反映すると、反映したあとに取得したプレイヤー情報のonboarding_statusはcompletedのままになる", func(t *testing.T) {
				env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
				require.NoError(t, env.Onboarding.ApplyFactionSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "SHE"))
				assert.Equal(t, "completed", playerInfoOf(t, env, playerID).View.Player.OnboardingStatus)
			})

			t.Run("onboarding_statusがcompletedのプレイヤーに表示名設定イベントを反映するとき", func(t *testing.T) {
				t.Run("警告ログが1件出る", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					logs := loggingtest.Capture(t)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Len(t, usecasetest.SelectWarnings(logs), 1)
				})

				t.Run("反映するプレイヤーのIDがd4b1c7e9-2a5f-4e83-9c06-7f1a3b5d8e20のとき、警告ログのplayer_idはd4b1c7e9-2a5f-4e83-9c06-7f1a3b5d8e20になる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					logs := loggingtest.Capture(t)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Equal(t, []string{"d4b1c7e9-2a5f-4e83-9c06-7f1a3b5d8e20"}, usecasetest.SelectWarningValues(logs, "player_id"))
				})

				t.Run("警告ログのcurrentはcompletedになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					logs := loggingtest.Capture(t)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Equal(t, []string{"completed"}, usecasetest.SelectWarningValues(logs, "current"))
				})

				t.Run("警告ログのtargetはname_setになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					logs := loggingtest.Capture(t)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Equal(t, []string{"name_set"}, usecasetest.SelectWarningValues(logs, "target"))
				})

				t.Run("反映前の表示名がゆうしゃで、表示名がゆうしゃ2のイベントを反映しても、反映したあとに取得したプレイヤー情報のnameはゆうしゃのままになる", func(t *testing.T) {
					env, playerID := newOnboardingEnv(t, usecasetest.StepsToCompleted)
					require.NoError(t, env.Onboarding.ApplyNameSet(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "ゆうしゃ2"))
					assert.Equal(t, usecasetest.ToPointer("ゆうしゃ"), playerInfoOf(t, env, playerID).View.Player.Name)
				})
			})
		})
	})
}
