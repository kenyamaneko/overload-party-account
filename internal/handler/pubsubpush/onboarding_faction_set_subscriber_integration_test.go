//go:build integration

package pubsubpush_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

func hasInitialFaction(want string) pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		pubsubpushtest.AssertInitialFaction(t, want, o.Stack.FetchPlayerView(t, o.PlayerID))
	}
}

func hasNoInitialFaction() pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Nil(t, o.Stack.FetchPlayerView(t, o.PlayerID).InitialFaction)
	}
}

func hasNoOwnedFactions() pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Empty(t, o.Stack.FetchOwnedFactions(t, o.PlayerID))
	}
}

func pushFactionSetForPlayer(t *testing.T, setup pubsubpushtest.PlayerSetup, faction string) pubsubpushtest.Outcome {
	t.Helper()
	return pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathOnboardingFactionSet, setup, func(t *testing.T, playerID string) string {
		return pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), playerID, faction)
	})
}

func TestOnboardingFactionSetApplication(t *testing.T) {
	t.Run("[オンボーディング陣営設定イベント]初期陣営の反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("陣営「SHE」のイベントを受けると", func(t *testing.T) {
				setupNameSetOwningShe := func(t *testing.T, s *pubsubpushtest.AccountStack) string {
					playerID := s.RegisterWithNameSet(t, "テスト太郎")
					s.PushAccepted(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), playerID, "SHE"))
					return playerID
				}
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					check pubsubpushtest.OutcomeCheck
				}{
					{"初期陣営が未確定で、オンボーディング進行状態が「name_set」のプレイヤーのとき、応答は200になる", pubsubpushtest.SetupNameSet, pubsubpushtest.RespondsOK()},
					{"初期陣営が未確定で、オンボーディング進行状態が「name_set」のプレイヤーのとき、取得したプレイヤー情報の初期陣営は「SHE」になる", pubsubpushtest.SetupNameSet, hasInitialFaction("SHE")},
					{"オンボーディング進行状態が「name_set」で、初期陣営ではないが「SHE」を所持しているプレイヤーのとき、取得したプレイヤー情報の初期陣営は「SHE」になる", setupNameSetOwningShe, hasInitialFaction("SHE")},
					{"初期陣営が「SHE」と確定済みのプレイヤーで、別のイベントIDのとき、応答は200になる", pubsubpushtest.SetupFactionSet, pubsubpushtest.RespondsOK()},
					{"初期陣営が「SHE」と確定済みのプレイヤーで、別のイベントIDのとき、取得したプレイヤー情報の初期陣営は「SHE」のままになる", pubsubpushtest.SetupFactionSet, hasInitialFaction("SHE")},
					{"オンボーディング進行状態が「completed」のプレイヤーで、別のイベントIDのとき、取得したプレイヤー情報のオンボーディング進行状態は「completed」のままになる", pubsubpushtest.SetupCompleted, hasOnboardingStatus("completed")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pushFactionSetForPlayer(t, tc.setup, "SHE")

						tc.check(t, o)
					})
				}
			})

			t.Run("初期陣営が未確定で、オンボーディング進行状態が「name_set」のプレイヤーに、陣営「SHE」のイベントを受けたとき", func(t *testing.T) {
				t.Run("取得した所持陣営に「SHE」が含まれる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithNameSet(t, "テスト太郎")

					stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), playerID, "SHE"))

					assert.Contains(t, stack.FetchOwnedFactions(t, playerID), "SHE")
				})

				t.Run("取得したプレイヤー情報のオンボーディング進行状態は「faction_set」になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithNameSet(t, "テスト太郎")

					stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), playerID, "SHE"))

					assert.Equal(t, "faction_set", stack.FetchPlayerView(t, playerID).Player.OnboardingStatus)
				})
			})

			t.Run("オンボーディング進行状態が「name_set」のプレイヤーに、1回目が陣営「SHE」のイベントで、同じイベントIDの2回目も陣営「SHE」のイベントを受けるとき", func(t *testing.T) {
				cases := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"取得したプレイヤー情報の初期陣営は「SHE」のままになる", hasInitialFaction("SHE")},
					{"取得した所持陣営は「SHE」の1件だけになる", pubsubpushtest.OwnsFactions("SHE")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						stack, logs := pubsubpushtest.NewCase(t, sharedPg)
						playerID := stack.RegisterWithNameSet(t, "テスト太郎")
						eventID := pubsubpushtest.NewEventID()
						stack.PushAccepted(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, eventID, playerID, "SHE"))

						rec := stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, eventID, playerID, "SHE"))

						tc.check(t, pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID})
					})
				}
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("オンボーディング進行状態が「name_set」のプレイヤーに、1回目が陣営「SHE」のイベントで、同じイベントIDの2回目が陣営「Tenki」のイベントを受けるとき", func(t *testing.T) {
				cases := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"取得したプレイヤー情報の初期陣営は「SHE」のままになる", hasInitialFaction("SHE")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						stack, logs := pubsubpushtest.NewCase(t, sharedPg)
						playerID := stack.RegisterWithNameSet(t, "テスト太郎")
						eventID := pubsubpushtest.NewEventID()
						stack.PushAccepted(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, eventID, playerID, "SHE"))

						rec := stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, eventID, playerID, "Tenki"))

						tc.check(t, pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID})
					})
				}
			})

			t.Run("陣営設定のイベントを受けると", func(t *testing.T) {
				respondsHandlerFailed := pubsubpushtest.CheckPushResult(pubsubpushtest.RespondsHandlerFailed())
				cases := []struct {
					name    string
					setup   pubsubpushtest.PlayerSetup
					faction string
					check   pubsubpushtest.OutcomeCheck
				}{
					{"該当プレイヤーが存在しないとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupUnregistered, "SHE", respondsHandlerFailed},
					{"オンボーディング進行状態が「name_set」のプレイヤーに、選択できない陣営「Neutral」のイベントのとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupNameSet, "Neutral", respondsHandlerFailed},
					{"オンボーディング進行状態が「name_set」のプレイヤーに、選択できない陣営「Neutral」のイベントのとき、取得したプレイヤー情報の初期陣営は未確定のままになる", pubsubpushtest.SetupNameSet, "Neutral", hasNoInitialFaction()},
					{"オンボーディング進行状態が「not_started」のプレイヤーに、表示名設定を経ずに陣営「SHE」のイベントのとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupRegistered, "SHE", respondsHandlerFailed},
					{"オンボーディング進行状態が「not_started」のプレイヤーに、表示名設定を経ずに陣営「SHE」のイベントのとき、取得したプレイヤー情報の初期陣営は未確定のままになる", pubsubpushtest.SetupRegistered, "SHE", hasNoInitialFaction()},
					{"オンボーディング進行状態が「not_started」のプレイヤーに、表示名設定を経ずに陣営「SHE」のイベントのとき、取得した所持陣営は空のままになる", pubsubpushtest.SetupRegistered, "SHE", hasNoOwnedFactions()},
					{"オンボーディング進行状態が「not_started」のプレイヤーに、表示名設定を経ずに陣営「SHE」のイベントのとき、取得したプレイヤー情報のオンボーディング進行状態は「not_started」のままになる", pubsubpushtest.SetupRegistered, "SHE", hasOnboardingStatus("not_started")},
					{"初期陣営が「SHE」と確定済みのプレイヤーに、陣営「Tenki」のイベントのとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupFactionSet, "Tenki", respondsHandlerFailed},
					{"初期陣営が「SHE」と確定済みのプレイヤーに、陣営「Tenki」のイベントのとき、取得したプレイヤー情報の初期陣営は「SHE」のままになる", pubsubpushtest.SetupFactionSet, "Tenki", hasInitialFaction("SHE")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pushFactionSetForPlayer(t, tc.setup, tc.faction)

						tc.check(t, o)
					})
				}
			})

			t.Run("該当プレイヤーが存在しないとき", func(t *testing.T) {
				t.Run("ERRORログのエラーの内容は、プレイヤー不在を示す", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.NewEventID(), uuid.NewString(), "SHE"))

					pubsubpushtest.AssertAnyErrorLogContains(t, capture, "error", "not found")
				})

				t.Run("イベントIDが「11111111-1111-4111-8111-111111111111」のとき、ERRORログのイベントIDは「11111111-1111-4111-8111-111111111111」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, "SHE"))

					pubsubpushtest.AssertAnyErrorLogAttr(t, capture, "event_id", "11111111-1111-4111-8111-111111111111")
				})

				t.Run("イベントのプレイヤーIDが「22222222-2222-4222-8222-222222222222」のとき、ERRORログのプレイヤーIDは「22222222-2222-4222-8222-222222222222」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpushtest.BuildFactionSetBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, "SHE"))

					pubsubpushtest.AssertAnyErrorLogAttr(t, capture, "player_id", "22222222-2222-4222-8222-222222222222")
				})
			})
		})
	})
}
