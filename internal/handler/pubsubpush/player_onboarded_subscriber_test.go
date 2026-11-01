package pubsubpush_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

type stubOnboardingCompletedApplier struct{}

func (stubOnboardingCompletedApplier) ApplyCompleted(_ context.Context, _, _, _ string) error {
	return nil
}

func validPlayerOnboardedEvent() map[string]any {
	return map[string]any{
		"event_type":         "player_onboarded",
		"event_id":           pubsubpushtest.DummyEventID,
		"timestamp":          "2026-01-01T00:00:00Z",
		"player_id":          pubsubpushtest.DummyPlayerID,
		"initial_faction_id": "SHE",
	}
}

func pushPlayerOnboardedWithStub(t *testing.T, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewPlayerOnboardedSubscriber(stubOnboardingCompletedApplier{})
	handler := pubsubpush.NewEventHandler(pubsubpush.PlayerOnboardedSubscription, subscriber.ReceivePlayerOnboarded)
	return pubsubpushtest.PushEvent(t, pubsubpushtest.NewPushEngine(pubsubpushtest.PushPathPlayerOnboarded, handler.ReceivePush), pubsubpushtest.PushPathPlayerOnboarded, eventBody)
}

func TestPlayerOnboardedPayloadValidation(t *testing.T) {
	t.Run("[オンボーディング完了イベント]受信内容の検証", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("検証に失敗するイベント本文をプッシュすると", func(t *testing.T) {
				base := validPlayerOnboardedEvent()
				cases := []struct {
					name      string
					eventBody string
					check     pubsubpushtest.PushCheck
				}{
					{"イベント本文がJSONでない「not-json」のとき、処理失敗を示す500の応答になる", "not-json", pubsubpushtest.RespondsHandlerFailed()},
					{"event_typeが「onboarding_name_set」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "onboarding_name_set"), pubsubpushtest.RespondsHandlerFailed()},
					{"player_idがUUID形式でない「not-a-uuid」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "player_id", "not-a-uuid"), pubsubpushtest.RespondsHandlerFailed()},
					{"event_idがUUID形式でない「not-a-uuid」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_id", "not-a-uuid"), pubsubpushtest.RespondsHandlerFailed()},
					{"イベント本文がJSONでない「not-json」のとき、ERRORログに「イベント本文不正」が記録される", "not-json", pubsubpushtest.LogsError("player-onboarded subscriber: bad payload")},
					{"event_typeが「onboarding_name_set」のとき、ERRORログに「イベント種別不明」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "onboarding_name_set"), pubsubpushtest.LogsError("player-onboarded subscriber: unknown event_type")},
					{"player_idがUUID形式でない「not-a-uuid」のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "player_id", "not-a-uuid"), pubsubpushtest.LogsError("player-onboarded subscriber: missing required field")},
					{"event_idがUUID形式でない「not-a-uuid」のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_id", "not-a-uuid"), pubsubpushtest.LogsError("player-onboarded subscriber: missing required field")},
					{"player_idがUUID形式でない「not-a-uuid」のとき、ERRORログのエラーの内容は、プレイヤーIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "player_id", "not-a-uuid"), pubsubpushtest.LogsErrorContaining("player-onboarded subscriber: missing required field", "player_id: invalid UUID format")},
					{"event_idがUUID形式でない「not-a-uuid」のとき、ERRORログのエラーの内容は、イベントIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "event_id", "not-a-uuid"), pubsubpushtest.LogsErrorContaining("player-onboarded subscriber: missing required field", "event_id: invalid UUID format")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs := loggingtest.Capture(t)

						rec := pushPlayerOnboardedWithStub(t, tc.eventBody)

						tc.check(t, rec, logs)
					})
				}
			})
		})
	})
}
