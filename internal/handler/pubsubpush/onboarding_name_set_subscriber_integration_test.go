//go:build integration

package pubsubpush_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

func assertName(t *testing.T, want string, view *domain.PlayerView) {
	t.Helper()
	require.NotNil(t, view.Player.Name)
	assert.Equal(t, want, *view.Player.Name)
}

func hasName(want string) pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assertName(t, want, o.Stack.FetchPlayerView(t, o.PlayerID))
	}
}

func hasNoName() pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Nil(t, o.Stack.FetchPlayerView(t, o.PlayerID).Player.Name)
	}
}

func pushNameSetForPlayer(t *testing.T, setup pubsubpushtest.PlayerSetup, displayName string) pubsubpushtest.Outcome {
	t.Helper()
	return pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathOnboardingNameSet, setup, func(t *testing.T, playerID string) string {
		return pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, displayName)
	})
}

func hasOnboardingStatus(want string) pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Equal(t, want, o.Stack.FetchPlayerView(t, o.PlayerID).Player.OnboardingStatus)
	}
}

func TestOnboardingNameSetApplication(t *testing.T) {
	t.Run("[オンボーディング表示名設定イベント]表示名の反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("表示名のイベントを受けると", func(t *testing.T) {
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					check pubsubpushtest.OutcomeCheck
				}{
					{"オンボーディング進行状態が「not_started」のプレイヤーのとき、応答は200になる", pubsubpushtest.SetupRegistered, pubsubpushtest.RespondsOK()},
					{"オンボーディング進行状態が「name_set」のプレイヤーで、別のイベントIDのイベントのとき、応答は200になる", pubsubpushtest.SetupNameSet, pubsubpushtest.RespondsOK()},
					{"オンボーディング進行状態が「faction_set」のプレイヤーで、別のイベントIDのイベントのとき、応答は200になる", pubsubpushtest.SetupFactionSet, pubsubpushtest.RespondsOK()},
					{"オンボーディング進行状態が「name_set」のプレイヤーで、別のイベントIDのイベントのとき、取得したプレイヤー情報のオンボーディング進行状態は「name_set」のままになる", pubsubpushtest.SetupNameSet, hasOnboardingStatus("name_set")},
					{"オンボーディング進行状態が「faction_set」のプレイヤーで、別のイベントIDのイベントのとき、取得したプレイヤー情報のオンボーディング進行状態は「faction_set」のままになる", pubsubpushtest.SetupFactionSet, hasOnboardingStatus("faction_set")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathOnboardingNameSet, tc.setup, func(t *testing.T, playerID string) string {
							return pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, "テスト次郎")
						})

						tc.check(t, o)
					})
				}
			})

			t.Run("オンボーディング進行状態が「not_started」のプレイヤーに、表示名「テスト太郎」のイベントを受けたとき", func(t *testing.T) {
				t.Run("取得したプレイヤー情報の表示名は「テスト太郎」になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.Register(t)

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, "テスト太郎"))

					assertName(t, "テスト太郎", stack.FetchPlayerView(t, playerID))
				})

				t.Run("取得したプレイヤー情報のオンボーディング進行状態は「name_set」になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.Register(t)

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, "テスト太郎"))

					assert.Equal(t, "name_set", stack.FetchPlayerView(t, playerID).Player.OnboardingStatus)
				})
			})

			t.Run("表示名が「テスト太郎」のプレイヤーに、別のイベントIDで表示名「テスト次郎」のイベントを受けたとき", func(t *testing.T) {
				t.Run("オンボーディング進行状態が「faction_set」のとき、取得したプレイヤー情報の表示名は「テスト太郎」のままになる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithFactionSet(t, "テスト太郎")

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, "テスト次郎"))

					assertName(t, "テスト太郎", stack.FetchPlayerView(t, playerID))
				})

				t.Run("オンボーディング進行状態が「name_set」のとき、取得したプレイヤー情報の表示名は「テスト次郎」になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithNameSet(t, "テスト太郎")

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), playerID, "テスト次郎"))

					assertName(t, "テスト次郎", stack.FetchPlayerView(t, playerID))
				})
			})

			t.Run("1回目が表示名「テスト太郎」のイベントで、同じイベントIDの2回目も表示名「テスト太郎」のイベントを、同じプレイヤー宛てに受けるとき", func(t *testing.T) {
				checks := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"取得したプレイヤー情報の表示名は「テスト太郎」のままになる", hasName("テスト太郎")},
				}
				for _, tc := range checks {
					t.Run(tc.name, func(t *testing.T) {
						stack, logs := pubsubpushtest.NewCase(t, sharedPg)
						playerID := stack.Register(t)
						eventID := pubsubpushtest.NewEventID()
						stack.PushAccepted(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, eventID, playerID, "テスト太郎"))

						rec := stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, eventID, playerID, "テスト太郎"))

						tc.check(t, pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID})
					})
				}
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("1回目が表示名「テスト太郎」のイベントで、同じイベントIDの2回目が表示名「テスト次郎」のイベントを、同じプレイヤー宛てに受けるとき", func(t *testing.T) {
				checks := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"取得したプレイヤー情報の表示名は「テスト太郎」のままになる", hasName("テスト太郎")},
				}
				for _, tc := range checks {
					t.Run(tc.name, func(t *testing.T) {
						stack, logs := pubsubpushtest.NewCase(t, sharedPg)
						playerID := stack.Register(t)
						eventID := pubsubpushtest.NewEventID()
						stack.PushAccepted(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, eventID, playerID, "テスト太郎"))

						rec := stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, eventID, playerID, "テスト次郎"))

						tc.check(t, pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID})
					})
				}
			})

			t.Run("表示名のイベントを受けると", func(t *testing.T) {
				respondsHandlerFailed := pubsubpushtest.CheckPushResult(pubsubpushtest.RespondsHandlerFailed())
				cases := []struct {
					name        string
					setup       pubsubpushtest.PlayerSetup
					displayName string
					check       pubsubpushtest.OutcomeCheck
				}{
					{"該当プレイヤーが存在しないとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupUnregistered, "テスト太郎", respondsHandlerFailed},
					{"表示名が未設定のプレイヤーに、イベントの表示名が半角スペース3つのとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupRegistered, "   ", respondsHandlerFailed},
					{"表示名が未設定のプレイヤーに、イベントの表示名が半角スペース3つのとき、取得したプレイヤー情報の表示名は未設定のままになる", pubsubpushtest.SetupRegistered, "   ", hasNoName()},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pushNameSetForPlayer(t, tc.setup, tc.displayName)

						tc.check(t, o)
					})
				}
			})

			t.Run("該当プレイヤーが存在しないとき", func(t *testing.T) {
				t.Run("ERRORログのエラーの内容は、プレイヤー不在を示す", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.NewEventID(), uuid.NewString(), "テスト太郎"))

					pubsubpushtest.AssertAnyErrorLogContains(t, capture, "error", "not found")
				})

				t.Run("イベントIDが「11111111-1111-4111-8111-111111111111」のとき、ERRORログのイベントIDは「11111111-1111-4111-8111-111111111111」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, "テスト太郎"))

					pubsubpushtest.AssertAnyErrorLogAttr(t, capture, "event_id", "11111111-1111-4111-8111-111111111111")
				})

				t.Run("イベントのプレイヤーIDが「22222222-2222-4222-8222-222222222222」のとき、ERRORログのプレイヤーIDは「22222222-2222-4222-8222-222222222222」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpushtest.BuildNameSetBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, "テスト太郎"))

					pubsubpushtest.AssertAnyErrorLogAttr(t, capture, "player_id", "22222222-2222-4222-8222-222222222222")
				})
			})
		})
	})
}
