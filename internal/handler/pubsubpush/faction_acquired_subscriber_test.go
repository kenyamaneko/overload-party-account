package pubsubpush_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

type stubFactionAcquiredApplier struct{}

func (stubFactionAcquiredApplier) ApplyFactionAcquired(_ context.Context, _, _, _, _ string) error {
	return nil
}

func validFactionAcquiredEvent() map[string]any {
	return map[string]any{
		"event_type": "faction_acquired",
		"event_id":   pubsubpushtest.DummyEventID,
		"timestamp":  "2026-01-01T00:00:00Z",
		"player_id":  pubsubpushtest.DummyPlayerID,
		"faction":    "Tenki",
	}
}

func pushFactionAcquiredWithStub(t *testing.T, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewFactionAcquiredSubscriber(stubFactionAcquiredApplier{})
	handler := pubsubpush.NewEventHandler(pubsubpush.FactionAcquiredSubscription, subscriber.ReceiveFactionAcquired)
	return pubsubpushtest.PushEvent(t, pubsubpushtest.NewPushEngine(pubsubpushtest.PushPathFactionAcquired, handler.ReceivePush), pubsubpushtest.PushPathFactionAcquired, eventBody)
}

func TestFactionAcquiredPayloadValidation(t *testing.T) {
	t.Run("[陣営獲得イベント]受信内容の検証", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("検証に失敗するイベント本文をプッシュすると", func(t *testing.T) {
				base := validFactionAcquiredEvent()
				cases := []struct {
					name      string
					eventBody string
					check     pubsubpushtest.PushCheck
				}{
					{"イベント本文がJSONでない「not-json」のとき、処理失敗を示す500の応答になる", "not-json", pubsubpushtest.RespondsHandlerFailed()},
					{"event_typeが「premium_updated」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "premium_updated"), pubsubpushtest.RespondsHandlerFailed()},
					{"イベント本文がJSONでない「not-json」のとき、ERRORログに「イベント本文不正」が記録される", "not-json", pubsubpushtest.LogsError("faction-acquired subscriber: bad payload")},
					{"event_typeが「premium_updated」のとき、ERRORログに「イベント種別不明」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "premium_updated"), pubsubpushtest.LogsError("faction-acquired subscriber: unknown event_type")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs := loggingtest.Capture(t)

						rec := pushFactionAcquiredWithStub(t, tc.eventBody)

						tc.check(t, rec, logs)
					})
				}
			})
		})
	})
}
