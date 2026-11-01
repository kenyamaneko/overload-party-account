package pubsubpush_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

type failingOnboardingApplier struct {
	err error
}

func (a failingOnboardingApplier) ApplyNameSet(_ context.Context, _, _, _, _ string) error {
	return a.err
}

func (a failingOnboardingApplier) ApplyFactionSet(_ context.Context, _, _, _, _ string) error {
	return a.err
}

func (a failingOnboardingApplier) ApplyCompleted(_ context.Context, _, _, _ string) error {
	return a.err
}

func pushEventToSubscription[E any](t *testing.T, path, subscription string, handle func(context.Context, E) error, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	handler := pubsubpush.NewEventHandler(subscription, handle)
	return pubsubpushtest.PushEvent(t, pubsubpushtest.NewPushEngine(path, handler.ReceivePush), path, eventBody)
}

func pushNameSetEventWithApplier(t *testing.T, applier failingOnboardingApplier) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewOnboardingNameSetSubscriber(applier)
	return pushEventToSubscription(t, pubsubpushtest.PushPathOnboardingNameSet, pubsubpush.OnboardingNameSetSubscription, subscriber.ReceiveOnboardingNameSet, pubsubpushtest.MarshalJSON(t, validOnboardingNameSetEvent()))
}

func pushFactionSetEventWithApplier(t *testing.T, applier failingOnboardingApplier) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewOnboardingFactionSetSubscriber(applier)
	return pushEventToSubscription(t, pubsubpushtest.PushPathOnboardingFactionSet, pubsubpush.OnboardingFactionSetSubscription, subscriber.ReceiveOnboardingFactionSet, pubsubpushtest.MarshalJSON(t, validOnboardingFactionSetEvent()))
}

func pushCompletedEventWithApplier(t *testing.T, applier failingOnboardingApplier) *httptest.ResponseRecorder {
	t.Helper()
	subscriber := pubsubpush.NewPlayerOnboardedSubscriber(applier)
	return pushEventToSubscription(t, pubsubpushtest.PushPathPlayerOnboarded, pubsubpush.PlayerOnboardedSubscription, subscriber.ReceivePlayerOnboarded, pubsubpushtest.MarshalJSON(t, validPlayerOnboardedEvent()))
}

func TestOnboardingProgressFailureClassificationByApplierError(t *testing.T) {
	t.Run("[オンボーディング進行イベント]失敗原因の分類", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("イベントを受けると", func(t *testing.T) {
				const (
					nameSetPublisherBugMessage         = "onboarding-name-set subscriber: publisher bug"
					nameSetApplyFailedMessage          = "onboarding-name-set subscriber: apply failed"
					factionSetPublisherBugMessage      = "onboarding-faction-set subscriber: publisher bug"
					factionSetApplyFailedMessage       = "onboarding-faction-set subscriber: apply failed"
					playerOnboardedPublisherBugMessage = "player-onboarded subscriber: publisher bug"
					playerOnboardedApplyFailedMessage  = "player-onboarded subscriber: apply failed"
				)
				cases := []struct {
					name    string
					push    func(t *testing.T, applier failingOnboardingApplier) *httptest.ResponseRecorder
					err     error
					message string
				}{
					{"表示名設定イベントの反映が、該当プレイヤーが存在しないために失敗するとき、ERRORログに「発行側起因の不整合」が記録される", pushNameSetEventWithApplier, port.ErrNotFound, nameSetPublisherBugMessage},
					{"陣営設定イベントの反映が、該当プレイヤーが存在しないために失敗するとき、ERRORログに「発行側起因の不整合」が記録される", pushFactionSetEventWithApplier, port.ErrNotFound, factionSetPublisherBugMessage},
					{"完了イベントの反映が、該当プレイヤーが存在しないために失敗するとき、ERRORログに「発行側起因の不整合」が記録される", pushCompletedEventWithApplier, port.ErrNotFound, playerOnboardedPublisherBugMessage},
					{"陣営設定イベントの反映が、イベントの陣営と異なる初期陣営が確定済みのために失敗するとき、ERRORログに「発行側起因の不整合」が記録される", pushFactionSetEventWithApplier, usecase.ErrFactionConflict, factionSetPublisherBugMessage},
					{"表示名設定イベントの反映が、表示名が不正なために失敗するとき、ERRORログに「発行側起因の不整合」が記録される", pushNameSetEventWithApplier, domain.ErrInvalidName, nameSetPublisherBugMessage},
					{"陣営設定イベントの反映が、陣営が選択できないために失敗するとき、ERRORログに「発行側起因の不整合」が記録される", pushFactionSetEventWithApplier, usecase.ErrInvalidFaction, factionSetPublisherBugMessage},
					{"陣営設定イベントの反映が、オンボーディング進行状態が段階を飛ばしているために失敗するとき、ERRORログに「反映失敗」が記録される", pushFactionSetEventWithApplier, domain.ErrOnboardingStatusSkipped, factionSetApplyFailedMessage},
					{"完了イベントの反映が、オンボーディング進行状態が段階を飛ばしているために失敗するとき、ERRORログに「反映失敗」が記録される", pushCompletedEventWithApplier, domain.ErrOnboardingStatusSkipped, playerOnboardedApplyFailedMessage},
					{"表示名設定イベントの反映が、データベースに接続できないために失敗するとき、ERRORログに「反映失敗」が記録される", pushNameSetEventWithApplier, errors.New("database connection refused"), nameSetApplyFailedMessage},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs := loggingtest.Capture(t)

						rec := tc.push(t, failingOnboardingApplier{err: tc.err})

						pubsubpushtest.LogsError(tc.message)(t, rec, logs)
					})
				}
			})
		})
	})
}
