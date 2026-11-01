package pubsubpush_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

type stubOnboardingNameSetApplier struct{}

func (stubOnboardingNameSetApplier) ApplyNameSet(_ context.Context, _, _, _, _ string) error {
	return nil
}

func validOnboardingNameSetEvent() map[string]any {
	return map[string]any{
		"event_type": "onboarding_name_set",
		"event_id":   pubsubpushtest.DummyEventID,
		"timestamp":  "2026-01-01T00:00:00Z",
		"player_id":  pubsubpushtest.DummyPlayerID,
		"name":       "テスト太郎",
	}
}

func pushOnboardingNameSetWithStub(t *testing.T, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewOnboardingNameSetSubscriber(stubOnboardingNameSetApplier{})
	handler := pubsubpush.NewEventHandler(pubsubpush.OnboardingNameSetSubscription, subscriber.ReceiveOnboardingNameSet)
	return pubsubpushtest.PushEvent(t, pubsubpushtest.NewPushEngine(pubsubpushtest.PushPathOnboardingNameSet, handler.ReceivePush), pubsubpushtest.PushPathOnboardingNameSet, eventBody)
}

func TestEventPayloadValidation(t *testing.T) {
	t.Run("[イベント受信]イベント本文の検証", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("オンボーディング表示名設定イベントの購読に、検証に失敗するイベント本文をプッシュすると", func(t *testing.T) {
				base := validOnboardingNameSetEvent()
				cases := []struct {
					name      string
					eventBody string
					check     pubsubpushtest.PushCheck
				}{
					{"イベント本文がJSONでない「not-json」のとき、処理失敗を示す500の応答になる", "not-json", pubsubpushtest.RespondsHandlerFailed()},
					{"event_idが数値「123」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_id", 123), pubsubpushtest.RespondsHandlerFailed()},
					{"event_typeが「onboarding_faction_set」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "onboarding_faction_set"), pubsubpushtest.RespondsHandlerFailed()},
					{"event_typeが空文字のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_type", ""), pubsubpushtest.RespondsHandlerFailed()},
					{"event_typeが無いとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWithout(t, base, "event_type"), pubsubpushtest.RespondsHandlerFailed()},
					{"player_idが空文字のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "player_id", ""), pubsubpushtest.RespondsHandlerFailed()},
					{"player_idが無いとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWithout(t, base, "player_id"), pubsubpushtest.RespondsHandlerFailed()},
					{"player_idがUUID形式でない「not-a-uuid」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "player_id", "not-a-uuid"), pubsubpushtest.RespondsHandlerFailed()},
					{"event_idがUUID形式でない「not-a-uuid」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_id", "not-a-uuid"), pubsubpushtest.RespondsHandlerFailed()},
					{"event_idが空文字のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "event_id", ""), pubsubpushtest.RespondsHandlerFailed()},
					{"event_idが無いとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWithout(t, base, "event_id"), pubsubpushtest.RespondsHandlerFailed()},
					{"nameが空文字のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "name", ""), pubsubpushtest.RespondsHandlerFailed()},
					{"nameが無いとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWithout(t, base, "name"), pubsubpushtest.RespondsHandlerFailed()},
					{"timestampが日時形式でない「tomorrow」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", "tomorrow"), pubsubpushtest.RespondsHandlerFailed()},
					{"timestampが数値「123」のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", 123), pubsubpushtest.RespondsHandlerFailed()},
					{"timestampが空文字のとき、処理失敗を示す500の応答になる", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", ""), pubsubpushtest.RespondsHandlerFailed()},
					{"イベント本文がJSONでない「not-json」のとき、ERRORログに「イベント本文不正」が記録される", "not-json", pubsubpushtest.LogsError("onboarding-name-set subscriber: bad payload")},
					{"event_idが数値「123」のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_id", 123), pubsubpushtest.LogsError("onboarding-name-set subscriber: bad payload")},
					{"event_typeが「onboarding_faction_set」のとき、ERRORログに「イベント種別不明」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "onboarding_faction_set"), pubsubpushtest.LogsError("onboarding-name-set subscriber: unknown event_type")},
					{"event_typeが空文字のとき、ERRORログに「イベント種別不明」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_type", ""), pubsubpushtest.LogsError("onboarding-name-set subscriber: unknown event_type")},
					{"event_typeが無いとき、ERRORログに「イベント種別不明」が記録される", pubsubpushtest.MarshalJSONWithout(t, base, "event_type"), pubsubpushtest.LogsError("onboarding-name-set subscriber: unknown event_type")},
					{"event_typeが「onboarding_faction_set」のとき、ERRORログに受信したイベント種別として「onboarding_faction_set」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_type", "onboarding_faction_set"), pubsubpushtest.LogsErrorAttr("onboarding-name-set subscriber: unknown event_type", "event_type", "onboarding_faction_set")},
					{"player_idが空文字のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "player_id", ""), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"player_idが無いとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWithout(t, base, "player_id"), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"player_idがUUID形式でない「not-a-uuid」のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "player_id", "not-a-uuid"), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"event_idがUUID形式でない「not-a-uuid」のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_id", "not-a-uuid"), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"event_idが空文字のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "event_id", ""), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"event_idが無いとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWithout(t, base, "event_id"), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"nameが空文字のとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "name", ""), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"nameが無いとき、ERRORログに「必須項目不足」が記録される", pubsubpushtest.MarshalJSONWithout(t, base, "name"), pubsubpushtest.LogsError("onboarding-name-set subscriber: missing required field")},
					{"timestampが日時形式でない「tomorrow」のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", "tomorrow"), pubsubpushtest.LogsError("onboarding-name-set subscriber: bad payload")},
					{"timestampが数値「123」のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", 123), pubsubpushtest.LogsError("onboarding-name-set subscriber: bad payload")},
					{"timestampが空文字のとき、ERRORログに「イベント本文不正」が記録される", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", ""), pubsubpushtest.LogsError("onboarding-name-set subscriber: bad payload")},
					{"イベント本文がJSONでない「not-json」のとき、ERRORログのエラーの内容は、イベント本文がJSONとして読めないことを示す", "not-json", pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: bad payload", "event payload is not valid JSON")},
					{"event_idが数値「123」のとき、ERRORログのエラーの内容は、event_idの型が契約と合わないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "event_id", 123), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: bad payload", "event payload type mismatch: event_id")},
					{"player_idが空文字のとき、ERRORログのエラーの内容は、プレイヤーIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "player_id", ""), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "player_id: invalid UUID format")},
					{"player_idが無いとき、ERRORログのエラーの内容は、プレイヤーIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWithout(t, base, "player_id"), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "player_id: invalid UUID format")},
					{"player_idがUUID形式でない「not-a-uuid」のとき、ERRORログのエラーの内容は、プレイヤーIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "player_id", "not-a-uuid"), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "player_id: invalid UUID format")},
					{"event_idがUUID形式でない「not-a-uuid」のとき、ERRORログのエラーの内容は、イベントIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "event_id", "not-a-uuid"), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "event_id: invalid UUID format")},
					{"event_idが空文字のとき、ERRORログのエラーの内容は、イベントIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "event_id", ""), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "event_id: invalid UUID format")},
					{"event_idが無いとき、ERRORログのエラーの内容は、イベントIDがUUID形式でないことを示す", pubsubpushtest.MarshalJSONWithout(t, base, "event_id"), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "event_id: invalid UUID format")},
					{"nameが空文字のとき、ERRORログのエラーの内容は、nameが空であることを示す", pubsubpushtest.MarshalJSONWith(t, base, "name", ""), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "name: required field is empty")},
					{"nameが無いとき、ERRORログのエラーの内容は、nameが空であることを示す", pubsubpushtest.MarshalJSONWithout(t, base, "name"), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: missing required field", "name: required field is empty")},
					{"timestampが日時形式でない「tomorrow」のとき、ERRORログのエラーの内容は、timestampが日時として読めないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", "tomorrow"), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: bad payload", "event payload date-time is invalid: timestamp")},
					{"timestampが数値「123」のとき、ERRORログのエラーの内容は、timestampの型が契約と合わないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", 123), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: bad payload", "event payload type mismatch: timestamp")},
					{"timestampが空文字のとき、ERRORログのエラーの内容は、timestampが日時として読めないことを示す", pubsubpushtest.MarshalJSONWith(t, base, "timestamp", ""), pubsubpushtest.LogsErrorContaining("onboarding-name-set subscriber: bad payload", "event payload date-time is invalid: timestamp")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs := loggingtest.Capture(t)

						rec := pushOnboardingNameSetWithStub(t, tc.eventBody)

						tc.check(t, rec, logs)
					})
				}
			})
		})
	})
}
