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

const (
	premiumFirstTimestamp  = "2026-01-01T00:00:00Z"
	premiumSecondTimestamp = "2026-01-02T00:00:00Z"

	premiumUpdatedFailedMessage = "premium-updated subscriber: handler failed"
)

func premiumUpdatedEvent(eventID, playerID, timestamp string, isPremium bool) map[string]any {
	return map[string]any{
		"event_type": "premium_updated",
		"event_id":   eventID,
		"timestamp":  timestamp,
		"player_id":  playerID,
		"is_premium": isPremium,
		"source":     "test-source",
	}
}

func premiumUpdatedBody(t *testing.T, eventID, playerID, timestamp string, isPremium bool) string {
	t.Helper()
	return pubsubpushtest.MarshalJSON(t, premiumUpdatedEvent(eventID, playerID, timestamp, isPremium))
}

func premiumUpdatedBodyWithExpiry(t *testing.T, eventID, playerID, timestamp string, isPremium bool, expiresAt string) string {
	t.Helper()
	return pubsubpushtest.MarshalJSONWith(t, premiumUpdatedEvent(eventID, playerID, timestamp, isPremium), "premium_expires_at", expiresAt)
}

func assertExpiresAt(t *testing.T, want string, view *domain.PlayerView) {
	t.Helper()
	require.NotNil(t, view.Player.PremiumExpiresAt)
	assert.Equal(t, want, view.Player.PremiumExpiresAt.UTC().Format("2006-01-02T15:04:05Z"))
}

func hasPremiumStatus(want bool) pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Equal(t, want, o.Stack.FetchPlayerView(t, o.PlayerID).Player.IsPremium)
	}
}

func hasExpiresAt(want string) pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assertExpiresAt(t, want, o.Stack.FetchPlayerView(t, o.PlayerID))
	}
}

func hasNoExpiresAt() pubsubpushtest.OutcomeCheck {
	return func(t *testing.T, o pubsubpushtest.Outcome) {
		t.Helper()
		assert.Nil(t, o.Stack.FetchPlayerView(t, o.PlayerID).Player.PremiumExpiresAt)
	}
}

func setupPremiumExpiring2030(t *testing.T, s *pubsubpushtest.AccountStack) string {
	t.Helper()
	playerID := s.Register(t)
	s.PushAccepted(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBodyWithExpiry(t, pubsubpushtest.NewEventID(), playerID, premiumFirstTimestamp, true, "2030-01-01T00:00:00Z"))
	return playerID
}

func registerWithPremiumReflectedAt2030(t *testing.T) (*pubsubpushtest.AccountStack, string) {
	t.Helper()
	stack, _ := pubsubpushtest.NewCase(t, sharedPg)
	playerID := stack.Register(t)
	stack.PushAccepted(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBodyWithExpiry(t, pubsubpushtest.NewEventID(), playerID, "2030-01-01T00:00:00Z", true, "2031-01-01T00:00:00Z"))
	return stack, playerID
}

type premiumUpdatedBodyBuilder func(t *testing.T, eventID, playerID string) string

func premiumFirst(t *testing.T, eventID, playerID string) string {
	return premiumUpdatedBody(t, eventID, playerID, premiumFirstTimestamp, true)
}

func premiumThenRevoked(t *testing.T, eventID, playerID string) string {
	return premiumUpdatedBody(t, eventID, playerID, premiumSecondTimestamp, false)
}

func premiumExpiring2030(t *testing.T, eventID, playerID string) string {
	return premiumUpdatedBodyWithExpiry(t, eventID, playerID, premiumFirstTimestamp, true, "2030-01-01T00:00:00Z")
}

func pushPremiumUpdatedTwiceWithSameEventID(t *testing.T, first, second premiumUpdatedBodyBuilder) pubsubpushtest.Outcome {
	t.Helper()
	stack, logs := pubsubpushtest.NewCase(t, sharedPg)
	playerID := stack.Register(t)
	eventID := pubsubpushtest.NewEventID()
	stack.PushAccepted(t, pubsubpushtest.PushPathPremiumUpdated, first(t, eventID, playerID))

	rec := stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, second(t, eventID, playerID))

	return pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID}
}

func TestPremiumUpdatedApplication(t *testing.T) {
	t.Run("[プレミアム状態更新イベント]プレミアム状態の反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("まだイベントを反映していない非プレミアムのプレイヤーに、is_premiumが真のイベントを受けるとき", func(t *testing.T) {
				expiring2030 := func(t *testing.T, eventID, playerID string) string {
					return premiumUpdatedBodyWithExpiry(t, eventID, playerID, premiumFirstTimestamp, true, "2030-01-01T00:00:00Z")
				}
				noExpiry := func(t *testing.T, eventID, playerID string) string {
					return premiumUpdatedBody(t, eventID, playerID, premiumFirstTimestamp, true)
				}
				timestampOneSecondAfterZeroTime := func(t *testing.T, eventID, playerID string) string {
					return premiumUpdatedBody(t, eventID, playerID, "0001-01-01T00:00:01Z", true)
				}
				cases := []struct {
					name  string
					body  func(t *testing.T, eventID, playerID string) string
					check pubsubpushtest.OutcomeCheck
				}{
					{"有効期限が2030-01-01T00:00:00Zのとき、応答は200になる", expiring2030, pubsubpushtest.RespondsOK()},
					{"有効期限が無いとき、応答は200になる", noExpiry, pubsubpushtest.RespondsOK()},
					{"発生時刻が0001-01-01T00:00:01Zのとき、応答は200になる", timestampOneSecondAfterZeroTime, pubsubpushtest.RespondsOK()},
					{"有効期限が2030-01-01T00:00:00Zのとき、取得したプレイヤー情報のプレミアム状態は有効になる", expiring2030, hasPremiumStatus(true)},
					{"有効期限が無いとき、取得したプレイヤー情報のプレミアム状態は有効になる", noExpiry, hasPremiumStatus(true)},
					{"発生時刻が0001-01-01T00:00:01Zのとき、取得したプレイヤー情報のプレミアム状態は有効になる", timestampOneSecondAfterZeroTime, hasPremiumStatus(true)},
					{"有効期限が2030-01-01T00:00:00Zのとき、取得したプレイヤー情報の有効期限は2030-01-01T00:00:00Zになる", expiring2030, hasExpiresAt("2030-01-01T00:00:00Z")},
					{"有効期限が無いとき、取得したプレイヤー情報の有効期限は未設定のままになる", noExpiry, hasNoExpiresAt()},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						stack, logs := pubsubpushtest.NewCase(t, sharedPg)
						playerID := stack.Register(t)

						rec := stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, tc.body(t, pubsubpushtest.NewEventID(), playerID))

						tc.check(t, pubsubpushtest.Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID})
					})
				}
			})

			t.Run("is_premiumが真・有効期限が2030-01-01T00:00:00Z・発生時刻が2026-01-01T00:00:00Zのイベントを反映済みのプレイヤーに、別のイベントIDで、発生時刻が2026-01-01T00:00:00Z・is_premiumが真・有効期限が2031-01-01T00:00:00Zのイベントを受けたとき、取得したプレイヤー情報の有効期限は2031-01-01T00:00:00Zになる", func(t *testing.T) {
				stack, _, playerID := pubsubpushtest.NewCaseWithSetup(t, sharedPg, setupPremiumExpiring2030)

				stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBodyWithExpiry(t, pubsubpushtest.NewEventID(), playerID, premiumFirstTimestamp, true, "2031-01-01T00:00:00Z"))

				assertExpiresAt(t, "2031-01-01T00:00:00Z", stack.FetchPlayerView(t, playerID))
			})

			t.Run("is_premiumが真・有効期限が2031-01-01T00:00:00Zで発生時刻が2030-01-01T00:00:00Zのイベントを反映済みのプレイヤーに、別のイベントIDで、is_premiumが偽で有効期限が無いイベントを受けるとき", func(t *testing.T) {
				const (
					older = "2029-12-31T23:59:59Z"
					same  = "2030-01-01T00:00:00Z"
				)
				cases := []struct {
					name      string
					timestamp string
					check     pubsubpushtest.OutcomeCheck
				}{
					{"発生時刻が2029-12-31T23:59:59Zで反映済みのイベントより古いとき、応答は200になる", older, pubsubpushtest.RespondsOK()},
					{"発生時刻が2030-01-01T00:00:00Zで反映済みのイベントと同じとき、応答は200になる", same, pubsubpushtest.RespondsOK()},
					{"発生時刻が2029-12-31T23:59:59Zで反映済みのイベントより古いとき、取得したプレイヤー情報のプレミアム状態は有効のままになる", older, hasPremiumStatus(true)},
					{"発生時刻が2029-12-31T23:59:59Zで反映済みのイベントより古いとき、取得したプレイヤー情報の有効期限は2031-01-01T00:00:00Zのままになる", older, hasExpiresAt("2031-01-01T00:00:00Z")},
					{"発生時刻が2030-01-01T00:00:00Zで反映済みのイベントと同じとき、取得したプレイヤー情報のプレミアム状態は無効になる", same, hasPremiumStatus(false)},
					{"発生時刻が2030-01-01T00:00:00Zで反映済みのイベントと同じとき、取得したプレイヤー情報の有効期限は未設定に戻る", same, hasNoExpiresAt()},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						stack, playerID := registerWithPremiumReflectedAt2030(t)

						rec := stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBody(t, pubsubpushtest.NewEventID(), playerID, tc.timestamp, false))

						tc.check(t, pubsubpushtest.Outcome{Stack: stack, Rec: rec, PlayerID: playerID})
					})
				}
			})

		})

		t.Run("1回目がis_premiumが真のイベントで、同じイベントIDのイベントを、同じプレイヤー宛てに2回受けるとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				cases := []struct {
					name   string
					first  premiumUpdatedBodyBuilder
					second premiumUpdatedBodyBuilder
					check  pubsubpushtest.OutcomeCheck
				}{
					{"2回目もis_premiumが真のとき、2回目の応答は200になる", premiumFirst, premiumFirst, pubsubpushtest.RespondsOK()},
					{"2回目もis_premiumが真のとき、取得したプレイヤー情報のプレミアム状態は有効のままになる", premiumFirst, premiumFirst, hasPremiumStatus(true)},
					{"2回目もis_premiumが真で、1回目も2回目も有効期限が2030-01-01T00:00:00Zのとき、取得したプレイヤー情報の有効期限は2030-01-01T00:00:00Zのままになる", premiumExpiring2030, premiumExpiring2030, hasExpiresAt("2030-01-01T00:00:00Z")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						tc.check(t, pushPremiumUpdatedTwiceWithSameEventID(t, tc.first, tc.second))
					})
				}
			})

			t.Run("異常系", func(t *testing.T) {
				cases := []struct {
					name  string
					check pubsubpushtest.OutcomeCheck
				}{
					{"2回目の発生時刻が1回目より新しくis_premiumが偽のとき、2回目の応答は200になる", pubsubpushtest.RespondsOK()},
					{"2回目の発生時刻が1回目より新しくis_premiumが偽のとき、取得したプレイヤー情報のプレミアム状態は有効のままになる", hasPremiumStatus(true)},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						tc.check(t, pushPremiumUpdatedTwiceWithSameEventID(t, premiumFirst, premiumThenRevoked))
					})
				}
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("該当プレイヤーがいないとき", func(t *testing.T) {
				t.Run("ERRORログのエラーの内容は、プレイヤーがいないことを示す", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBody(t, pubsubpushtest.NewEventID(), uuid.NewString(), premiumFirstTimestamp, true))

					pubsubpushtest.AssertErrorLogContains(t, capture, premiumUpdatedFailedMessage, "not found")
				})

				t.Run("イベントIDが「11111111-1111-4111-8111-111111111111」のとき、ERRORログのイベントIDは「11111111-1111-4111-8111-111111111111」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, premiumFirstTimestamp, true))

					pubsubpushtest.AssertErrorLogAttr(t, capture, premiumUpdatedFailedMessage, "event_id", "11111111-1111-4111-8111-111111111111")
				})

				t.Run("イベントのプレイヤーIDが「22222222-2222-4222-8222-222222222222」のとき、ERRORログのプレイヤーIDは「22222222-2222-4222-8222-222222222222」になる", func(t *testing.T) {
					stack, capture := pubsubpushtest.NewCase(t, sharedPg)

					stack.Push(t, pubsubpushtest.PushPathPremiumUpdated, premiumUpdatedBody(t, pubsubpushtest.FixedEventID, pubsubpushtest.UnregisteredPlayerID, premiumFirstTimestamp, true))

					pubsubpushtest.AssertErrorLogAttr(t, capture, premiumUpdatedFailedMessage, "player_id", "22222222-2222-4222-8222-222222222222")
				})
			})

			t.Run("イベントを受けると", func(t *testing.T) {
				validBody := func(t *testing.T, playerID string) string {
					return premiumUpdatedBody(t, pubsubpushtest.NewEventID(), playerID, premiumFirstTimestamp, true)
				}
				malformedEventID := func(t *testing.T, playerID string) string {
					return premiumUpdatedBody(t, "not-a-uuid", playerID, premiumFirstTimestamp, true)
				}
				responds500 := pubsubpushtest.CheckPushResult(pubsubpushtest.RespondsHandlerFailed())
				failed := pubsubpushtest.CheckPushResult(pubsubpushtest.LogsError(premiumUpdatedFailedMessage))
				cases := []struct {
					name  string
					setup pubsubpushtest.PlayerSetup
					body  func(t *testing.T, playerID string) string
					check pubsubpushtest.OutcomeCheck
				}{
					{"該当プレイヤーがいないとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupUnregistered, validBody, responds500},
					{"イベントIDがUUID形式でない「not-a-uuid」のとき、処理失敗を示す500の応答になる", pubsubpushtest.SetupRegistered, malformedEventID, responds500},
					{"該当プレイヤーがいないとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.SetupUnregistered, validBody, failed},
					{"イベントIDがUUID形式でない「not-a-uuid」のとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.SetupRegistered, malformedEventID, failed},
					{"イベントIDがUUID形式でない「not-a-uuid」のとき、ERRORログのエラーの内容は、イベントIDがUUID形式でないことを示す", pubsubpushtest.SetupRegistered, malformedEventID, pubsubpushtest.CheckPushResult(pubsubpushtest.LogsErrorContaining(premiumUpdatedFailedMessage, "event_id: invalid UUID format"))},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathPremiumUpdated, tc.setup, tc.body)

						tc.check(t, o)
					})
				}
			})

			t.Run("プレミアム中のプレイヤーにイベントを受けるとき", func(t *testing.T) {
				missingIsPremium := func(t *testing.T, playerID string) string {
					return pubsubpushtest.MarshalJSONWithout(t, premiumUpdatedEvent(pubsubpushtest.NewEventID(), playerID, premiumSecondTimestamp, true), "is_premium")
				}
				nullIsPremium := func(t *testing.T, playerID string) string {
					return pubsubpushtest.MarshalJSONWith(t, premiumUpdatedEvent(pubsubpushtest.NewEventID(), playerID, premiumSecondTimestamp, true), "is_premium", nil)
				}
				cases := []struct {
					name  string
					body  func(t *testing.T, playerID string) string
					check pubsubpushtest.OutcomeCheck
				}{
					{"is_premiumが無いとき、取得したプレイヤー情報のプレミアム状態は有効のままになる", missingIsPremium, hasPremiumStatus(true)},
					{"is_premiumがnullのとき、取得したプレイヤー情報のプレミアム状態は有効のままになる", nullIsPremium, hasPremiumStatus(true)},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						o := pubsubpushtest.PushEventForPlayer(t, sharedPg, pubsubpushtest.PushPathPremiumUpdated, setupPremiumExpiring2030, tc.body)

						tc.check(t, o)
					})
				}
			})
		})
	})
}
