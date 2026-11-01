//go:build integration

package pubsubpush_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
)

const factionAcquiredFailedMessage = "faction-acquired subscriber: handler failed"

func newCaseWithPlayerID(t *testing.T, playerID string) (*pubsubpushtest.AccountStack, *loggingtest.Logs) {
	t.Helper()
	db := sharedPg.NewDatabase(t)
	stack := pubsubpushtest.NewAccountStack(t, db.Pool, porttest.NewFixedPlayerIDGenerator(playerID))
	return stack, loggingtest.Capture(t)
}

func ownsNoFactions() pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Empty(t, o.Stack.FetchOwnedFactions(t, o.PlayerID))
	}
}

func ownsFaction(want string) pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Contains(t, o.Stack.FetchOwnedFactions(t, o.PlayerID), want)
	}
}

func pushFactionAcquiredWithNewEventIDAfterSameEventIDTwice(t *testing.T, secondFaction string) pubsubpushtest.Outcome {
	t.Helper()
	o := pushFactionAcquiredTwiceWithSameEventID(t, secondFaction)
	o.Rec = o.Stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), o.PlayerID, secondFaction))
	return o
}

func pushFactionAcquiredTwiceWithSameEventID(t *testing.T, secondFaction string) pubsubpushtest.Outcome {
	t.Helper()
	stack, logs := pubsubpushtest.NewCase(t, sharedPg)
	playerID := stack.Register(t)
	eventID := pubsubpushtest.NewEventID()
	stack.PushAccepted(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, eventID, playerID, "Tenki"))

	rec := stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, eventID, playerID, secondFaction))

	return pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID}
}

func TestFactionAcquiredApplication(t *testing.T) {
	t.Run("[陣営獲得イベント]陣営の追加", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("陣営「Tenki」の獲得イベントを受けると", func(t *testing.T) {
				setupOwningTenki := func(t *testing.T, s *pubsubpushtest.AccountStack) string {
					playerID := s.Register(t)
					s.PushAccepted(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), playerID, "Tenki"))
					return playerID
				}
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					check pubsubpushtest.OutcomeCheck
				}{
					{"イベントの陣営が未所持のとき、応答は200になる", pubsubpushtest.SetupRegistered, pubsubpushtest.RespondsOK()},
					{"イベントの陣営が所持済みで、別のイベントIDのイベントのとき、応答は200になる", setupOwningTenki, pubsubpushtest.RespondsOK()},
					{"イベントの陣営が未所持のとき、取得した所持陣営に「Tenki」が含まれる", pubsubpushtest.SetupRegistered, ownsFaction("Tenki")},
					{"イベントの陣営が所持済みで、別のイベントIDのイベントのとき、取得した所持陣営は「Tenki」の1件のままになる", setupOwningTenki, pubsubpushtest.OwnsFactions("Tenki")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathFactionAcquired, tc.setup, func(t *testing.T, playerID string) string {
							return pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), playerID, "Tenki")
						})

						tc.check(t, o)
					})
				}
			})

			t.Run("初期陣営が「SHE」と確定済みのプレイヤーが獲得イベントを受けるとき", func(t *testing.T) {
				cases := []struct {
					name    string
					faction string
				}{
					{"イベントの陣営が「Tenki」のとき、取得したプレイヤー情報の初期陣営は「SHE」のままになる", "Tenki"},
					{"イベントの陣営が「SHE」のとき、取得したプレイヤー情報の初期陣営は「SHE」のままになる", "SHE"},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						stack, _ := pubsubpushtest.NewCase(t, sharedPg)
						playerID := stack.RegisterWithFactionSet(t, "テスト太郎")

						stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), playerID, tc.faction))

						pubsubpushtest.AssertInitialFaction(t, "SHE", stack.FetchPlayerView(t, playerID))
					})
				}
			})

		})

		t.Run("1回目が陣営「Tenki」のイベントで、同じイベントIDのイベントを、同じプレイヤー宛てに2回受けるとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				cases := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の陣営が「Tenki」のとき、2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"2回目の陣営が「Tenki」のとき、取得した所持陣営は「Tenki」の1件だけになる", pubsubpushtest.OwnsFactions("Tenki")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						tc.check(t, pushFactionAcquiredTwiceWithSameEventID(t, "Tenki"))
					})
				}
			})

			t.Run("異常系", func(t *testing.T) {
				cases := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の陣営が「Sugar」のとき、2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"2回目の陣営が「Sugar」のとき、取得した所持陣営は「Tenki」の1件だけになる", pubsubpushtest.OwnsFactions("Tenki")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						tc.check(t, pushFactionAcquiredTwiceWithSameEventID(t, "Sugar"))
					})
				}

				t.Run("2回目の陣営が「Sugar」で反映されなかった後、別のイベントIDで陣営「Sugar」のイベントを受けたとき", func(t *testing.T) {
					recoveryCases := []struct {
						name  string
						check pubsubpushtest.OutcomeCheck
					}{
						{"別のイベントIDのイベントの応答は200になる", pubsubpushtest.RespondsOK()},
						{"取得した所持陣営に「Sugar」が含まれる", ownsFaction("Sugar")},
					}
					for _, tc := range recoveryCases {
						t.Run(tc.name, func(t *testing.T) {
							tc.check(t, pushFactionAcquiredWithNewEventIDAfterSameEventIDTwice(t, "Sugar"))
						})
					}
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("該当プレイヤーがいないとき", func(t *testing.T) {
				t.Run("ERRORログのエラーの内容は、プレイヤーがいないことを示す", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), uuid.NewString(), "Tenki"))

					pubsubpushtest.AssertErrorLogContains(t, capture, factionAcquiredFailedMessage, "not found")
				})

				t.Run("イベントIDが「11111111-1111-4111-8111-111111111111」のとき、ERRORログのイベントIDは「11111111-1111-4111-8111-111111111111」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, "Tenki"))

					pubsubpushtest.AssertErrorLogAttr(t, capture, factionAcquiredFailedMessage, "event_id", "11111111-1111-4111-8111-111111111111")
				})

				t.Run("イベントのプレイヤーIDが「22222222-2222-4222-8222-222222222222」のとき、ERRORログのプレイヤーIDは「22222222-2222-4222-8222-222222222222」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, "Tenki"))

					pubsubpushtest.AssertErrorLogAttr(t, capture, factionAcquiredFailedMessage, "player_id", "22222222-2222-4222-8222-222222222222")
				})

				t.Run("応答が500になった後、プレイヤーが登録されてから、同じイベントIDの陣営「Tenki」の獲得イベントが再配信されたとき", func(t *testing.T) {
					playerID := uuid.NewString()
					eventID := pubsubpushtest.NewEventID()

					t.Run("再配信された獲得イベントの応答は200になる", func(t *testing.T) {
						stack, _ := newCaseWithPlayerID(t, playerID)
						stack.PushRejected(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, eventID, playerID, "Tenki"))
						stack.Register(t)

						rec := stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, eventID, playerID, "Tenki"))

						pubsubpushtest.AssertStatus(t, http.StatusOK, rec)
					})

					t.Run("取得した所持陣営に「Tenki」が含まれる", func(t *testing.T) {
						stack, _ := newCaseWithPlayerID(t, playerID)
						stack.PushRejected(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, eventID, playerID, "Tenki"))
						stack.Register(t)

						stack.Push(t, pubsubpushtest.PushPathFactionAcquired, pubsubpushtest.BuildFactionAcquiredBody(t, eventID, playerID, "Tenki"))

						assert.Contains(t, stack.FetchOwnedFactions(t, playerID), "Tenki")
					})
				})
			})

			t.Run("イベントを受けると", func(t *testing.T) {
				validBody := func(t *testing.T, playerID string) string {
					return pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), playerID, "Tenki")
				}
				unknownFaction := func(t *testing.T, playerID string) string {
					return pubsubpushtest.BuildFactionAcquiredBody(t, pubsubpushtest.NewEventID(), playerID, "Unknown")
				}
				malformedEventID := func(t *testing.T, playerID string) string {
					return pubsubpushtest.BuildFactionAcquiredBody(t, "not-a-uuid", playerID, "Tenki")
				}
				responds500 := pubsubpushtest.CheckPushResult(pubsubpushtest.RespondsHandlerFailed())
				failed := pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(factionAcquiredFailedMessage))
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					body  func(t *testing.T, playerID string) string
					check pubsubpushtest.OutcomeCheck
				}{
					{"該当プレイヤーがいないとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupUnregistered, validBody, responds500},
					{"陣営が「Unknown」のとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupRegistered, unknownFaction, responds500},
					{"イベントIDがUUID形式でない「not-a-uuid」のとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupRegistered, malformedEventID, responds500},
					{"該当プレイヤーがいないとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.SetupUnregistered, validBody, failed},
					{"陣営が「Unknown」のとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.SetupRegistered, unknownFaction, failed},
					{"イベントIDがUUID形式でない「not-a-uuid」のとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.SetupRegistered, malformedEventID, failed},
					{"陣営が「Unknown」のとき、ERRORログのエラーの内容は、陣営不正を示す", pubsubpushtest.SetupRegistered, unknownFaction, pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(factionAcquiredFailedMessage, "unknown faction"))},
					{"イベントIDがUUID形式でない「not-a-uuid」のとき、ERRORログのエラーの内容は、イベントIDがUUID形式でないことを示す", pubsubpushtest.SetupRegistered, malformedEventID, pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(factionAcquiredFailedMessage, "event_id: invalid UUID format"))},
					{"陣営が「Unknown」で、イベントの前の所持陣営が空のとき、取得した所持陣営は空のままになる", pubsubpushtest.SetupRegistered, unknownFaction, ownsNoFactions()},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathFactionAcquired, tc.setup, tc.body)

						tc.check(t, o)
					})
				}
			})
		})
	})
}
