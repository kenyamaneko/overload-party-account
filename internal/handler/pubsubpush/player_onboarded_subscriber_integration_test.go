//go:build integration

package pubsubpush_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

func pushOnboardedForPlayer(t *testing.T, setup pubsubpushtest.PlayerSetup) pubsubpushtest.Outcome {
	t.Helper()
	return pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathPlayerOnboarded, setup, func(t *testing.T, playerID string) string {
		return pubsubpushtest.BuildOnboardedBody(t, pubsubpushtest.NewEventID(), playerID)
	})
}

func TestPlayerOnboardedApplication(t *testing.T) {
	t.Run("[オンボーディング完了イベント]オンボーディング完了の反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("完了のイベントを受けると", func(t *testing.T) {
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					check pubsubpushtest.OutcomeCheck
				}{
					{"オンボーディング進行状態が「faction_set」のプレイヤーのとき、応答は200になる", pubsubpushtest.SetupFactionSet, pubsubpushtest.RespondsOK()},
					{"オンボーディング進行状態が「completed」のプレイヤーで、別のイベントIDのイベントのとき、応答は200になる", pubsubpushtest.SetupCompleted, pubsubpushtest.RespondsOK()},
					{"オンボーディング進行状態が「faction_set」のプレイヤーのとき、取得したプレイヤー情報のオンボーディング進行状態は「completed」になる", pubsubpushtest.SetupFactionSet, hasOnboardingStatus("completed")},
					{"オンボーディング進行状態が「completed」のプレイヤーで、別のイベントIDのイベントのとき、取得したプレイヤー情報のオンボーディング進行状態は「completed」のままになる", pubsubpushtest.SetupCompleted, hasOnboardingStatus("completed")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pushOnboardedForPlayer(t, tc.setup)

						tc.check(t, o)
					})
				}
			})

			t.Run("オンボーディング進行状態が「faction_set」のプレイヤーに、同じイベントIDの完了のイベントを2回受けるとき", func(t *testing.T) {
				t.Run("2回目の応答は200になる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithFactionSet(t, "テスト太郎")
					eventID := pubsubpushtest.NewEventID()
					stack.PushAccepted(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, eventID, playerID))

					rec := stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, eventID, playerID))

					pubsubpushtest.AssertStatus(t, http.StatusOK, rec)
				})

				t.Run("取得したプレイヤー情報のオンボーディング進行状態は「completed」のままになる", func(t *testing.T) {
					stack, _ := pubsubpushtest.NewCase(t, sharedPg)
					playerID := stack.RegisterWithFactionSet(t, "テスト太郎")
					eventID := pubsubpushtest.NewEventID()
					stack.PushAccepted(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, eventID, playerID))

					stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, eventID, playerID))

					assert.Equal(t, "completed", stack.FetchPlayerView(t, playerID).Player.OnboardingStatus)
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("完了のイベントを受けると", func(t *testing.T) {
				respondsHandlerFailed := pubsubpushtest.CheckPushResult(pubsubpushtest.RespondsHandlerFailed())
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					check pubsubpushtest.OutcomeCheck
				}{
					{"該当プレイヤーが存在しないとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupUnregistered, respondsHandlerFailed},
					{"オンボーディング進行状態が「name_set」のプレイヤーで、陣営設定を経ていないとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupNameSet, respondsHandlerFailed},
					{"オンボーディング進行状態が「name_set」のプレイヤーで、陣営設定を経ていないとき、取得したプレイヤー情報のオンボーディング進行状態は「name_set」のままになる", pubsubpushtest.SetupNameSet, hasOnboardingStatus("name_set")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pushOnboardedForPlayer(t, tc.setup)

						tc.check(t, o)
					})
				}
			})

			t.Run("該当プレイヤーが存在しないとき", func(t *testing.T) {
				t.Run("ERRORログのエラーの内容は、プレイヤー不在を示す", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, pubsubpushtest.NewEventID(), uuid.NewString()))

					pubsubpushtest.AssertAnyErrorLogContains(t, capture, "error", "not found")
				})

				t.Run("イベントIDが「11111111-1111-4111-8111-111111111111」のとき、ERRORログのイベントIDは「11111111-1111-4111-8111-111111111111」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID))

					pubsubpushtest.AssertAnyErrorLogAttr(t, capture, "event_id", "11111111-1111-4111-8111-111111111111")
				})

				t.Run("イベントのプレイヤーIDが「22222222-2222-4222-8222-222222222222」のとき、ERRORログのプレイヤーIDは「22222222-2222-4222-8222-222222222222」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID))

					pubsubpushtest.AssertAnyErrorLogAttr(t, capture, "player_id", "22222222-2222-4222-8222-222222222222")
				})
			})

			t.Run("オンボーディング進行状態が「faction_set」のプレイヤーが完了のイベントを処理したあと、オンボーディング進行状態が「faction_set」の別のプレイヤー宛てに、同じイベントIDの完了のイベントを受けたとき", func(t *testing.T) {
				pushProcessedEventToOtherPlayer := func(t *testing.T) pubsubpushtest.Outcome {
					t.Helper()
					stack, logs := pubsubpushtest.NewCase(t, sharedPg)
					processedEventID := pubsubpushtest.NewEventID()
					firstPlayerID := stack.RegisterWithFactionSet(t, "テスト太郎")
					stack.PushAccepted(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, processedEventID, firstPlayerID))
					otherPlayerID := stack.RegisterWithFactionSet(t, "テスト次郎")
					rec := stack.Push(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpushtest.BuildOnboardedBody(t, processedEventID, otherPlayerID))
					return pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: otherPlayerID}
				}

				t.Run("応答は200になる", func(t *testing.T) {
					pubsubpushtest.RespondsOK()(t, pushProcessedEventToOtherPlayer(t))
				})

				t.Run("別のプレイヤーの取得したプレイヤー情報のオンボーディング進行状態は「faction_set」のままになる", func(t *testing.T) {
					hasOnboardingStatus("faction_set")(t, pushProcessedEventToOtherPlayer(t))
				})
			})
		})
	})
}
