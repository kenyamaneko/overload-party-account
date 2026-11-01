package pubsubpush_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

type stubPremiumUpdatedApplier struct{}

func (stubPremiumUpdatedApplier) ApplyPremiumUpdated(_ context.Context, _, _, _ string, _ bool, _ *time.Time, _ time.Time) error {
	return nil
}

func validPremiumUpdatedEvent() map[string]any {
	return map[string]any{
		"event_type":         "premium_updated",
		"event_id":           pubsubpushtest.DummyEventID,
		"timestamp":          "2026-01-01T00:00:00Z",
		"player_id":          pubsubpushtest.DummyPlayerID,
		"is_premium":         true,
		"premium_expires_at": "2030-01-01T00:00:00Z",
		"source":             "test-source",
	}
}

func pushPremiumUpdatedWithStub(t *testing.T, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewPremiumUpdatedSubscriber(stubPremiumUpdatedApplier{})
	handler := pubsubpush.NewEventHandler(pubsubpush.PremiumUpdatedSubscription, subscriber.ReceivePremiumUpdated)
	return pubsubpushtest.PushEvent(t, pubsubpushtest.NewPushEngine(pubsubpushtest.PushPathPremiumUpdated, handler.ReceivePush), pubsubpushtest.PushPathPremiumUpdated, eventBody)
}

func TestPremiumUpdatedPayloadValidation(t *testing.T) {
	t.Run("[プレミアム状態更新イベント]受信内容の検証", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("検証に失敗するイベント本文をプッシュすると", func(t *testing.T) {
				base := validPremiumUpdatedEvent()
				cases := []struct {
					name      string
					eventBody string
					check     pubsubpushtest.PushCheck
				}{
					{"イベント本文がJSONでない「not-json」のとき、処理失敗を示す500の応答になる", "not-json", pubsubpushtest.RespondsHandlerFailed()},
					{"is_premiumが真偽値でない「yes」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "is_premium", "yes"), pubsubpushtest.RespondsHandlerFailed()},
					{"premium_expires_atが日時形式でない「tomorrow」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", "tomorrow"), pubsubpushtest.RespondsHandlerFailed()},
					{"premium_expires_atが数値「123」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", 123), pubsubpushtest.RespondsHandlerFailed()},
					{"premium_expires_atが空文字のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", ""), pubsubpushtest.RespondsHandlerFailed()},
					{"timestampが無いとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWithout(t, base, "timestamp"), pubsubpushtest.RespondsHandlerFailed()},
					{"is_premiumが無いとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWithout(t, base, "is_premium"), pubsubpushtest.RespondsHandlerFailed()},
					{"is_premiumがnullのとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "is_premium", nil), pubsubpushtest.RespondsHandlerFailed()},
					{"event_typeが「faction_acquired」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "faction_acquired"), pubsubpushtest.RespondsHandlerFailed()},
					{"イベント本文がJSONでない「not-json」のとき、ERRORログに「イベント本文不正」が記録される", "not-json", pubsubpushtest.LogsError("premium-updated subscriber: bad payload")},
					{"is_premiumが真偽値でない「yes」のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "is_premium", "yes"), pubsubpushtest.LogsError("premium-updated subscriber: bad payload")},
					{"premium_expires_atが日時形式でない「tomorrow」のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", "tomorrow"), pubsubpushtest.LogsError("premium-updated subscriber: bad payload")},
					{"premium_expires_atが数値「123」のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", 123), pubsubpushtest.LogsError("premium-updated subscriber: bad payload")},
					{"premium_expires_atが空文字のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", ""), pubsubpushtest.LogsError("premium-updated subscriber: bad payload")},
					{"timestampが無いとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.MarshalJSONWithout(t, base, "timestamp"), pubsubpushtest.LogsError("premium-updated subscriber: handler failed")},
					{"is_premiumが無いとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.MarshalJSONWithout(t, base, "is_premium"), pubsubpushtest.LogsError("premium-updated subscriber: handler failed")},
					{"is_premiumがnullのとき、ERRORログに「処理失敗」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "is_premium", nil), pubsubpushtest.LogsError("premium-updated subscriber: handler failed")},
					{"event_typeが「faction_acquired」のとき、ERRORログに「イベント種別不明」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "faction_acquired"), pubsubpushtest.LogsError("premium-updated subscriber: unknown event_type")},
					{"is_premiumが真偽値でない「yes」のとき、ERRORログのエラーの内容は、is_premiumの型が契約と合わないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "is_premium", "yes"), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: bad payload", "event payload type mismatch: is_premium")},
					{"premium_expires_atが日時形式でない「tomorrow」のとき、ERRORログのエラーの内容は、premium_expires_atが日時として読めないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", "tomorrow"), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: bad payload", "event payload date-time is invalid: premium_expires_at")},
					{"premium_expires_atが数値「123」のとき、ERRORログのエラーの内容は、premium_expires_atの型が契約と合わないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", 123), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: bad payload", "event payload type mismatch: premium_expires_at")},
					{"premium_expires_atが空文字のとき、ERRORログのエラーの内容は、premium_expires_atが日時として読めないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "premium_expires_at", ""), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: bad payload", "event payload date-time is invalid: premium_expires_at")},
					{"timestampが無いとき、ERRORログのエラーの内容は、timestampが無いことを示す", pubsubpushtest.MarshalJSONWithout(t, base, "timestamp"), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: handler failed", "timestamp is missing")},
					{"is_premiumが無いとき、ERRORログのエラーの内容は、is_premiumが無いことを示す", pubsubpushtest.MarshalJSONWithout(t, base, "is_premium"), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: handler failed", "is_premium is missing")},
					{"is_premiumがnullのとき、ERRORログのエラーの内容は、is_premiumが無いことを示す", pubsubpushtest.MarshalJSONWith(t, base, "is_premium", nil), pubsubpushtest.LogsErrorContaining("premium-updated subscriber: handler failed", "is_premium is missing")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs := loggingtest.Capture(t)

						rec := pushPremiumUpdatedWithStub(t, tc.eventBody)

						tc.check(t, rec, logs)
					})
				}
			})
		})
	})
}
