package pubsub

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// fakeOnboardingNameSetApplier は OnboardingNameSetApplier を満たす最小スタブ。
// subscriber は「usecase 層にイベント内容を正しく委譲し、戻り値に応じて
// 成功 / 失敗ログ分岐だけを行う」契約なので、usecase 内部の Tx / repo
// 挙動はここで抽象化する。
type fakeOnboardingNameSetApplier struct {
	returnProcessed bool
	returnErr       error

	called    bool
	gotEvent  string
	gotType   string
	gotPlayer string
	gotName   string
}

func (f *fakeOnboardingNameSetApplier) ApplyNameSet(
	_ context.Context,
	eventID, eventType, playerID, name string,
) (bool, error) {
	f.called = true
	f.gotEvent = eventID
	f.gotType = eventType
	f.gotPlayer = playerID
	f.gotName = name
	if f.returnErr != nil {
		return false, f.returnErr
	}
	return f.returnProcessed, nil
}

func TestHandleMessageOnboardingNameSet(t *testing.T) {
	t.Run("onboarding_name_setイベントの処理", func(t *testing.T) {
		const validEventID = "11111111-1111-1111-1111-111111111111"
		validPayload := mustMarshal(t, apiscenario.OnboardingNameSetEvent{
			EventType: apiscenario.EventTypeOnboardingNameSet,
			EventID:   validEventID,
			PlayerID:  "p-1",
			Name:      "Kenya",
		})

		tests := []struct {
			name            string
			payload         []byte
			returnProcessed bool
			returnErr       error
			wantErr         bool
			wantErrContains string
			assertApplier   func(t *testing.T, a *fakeOnboardingNameSetApplier)
		}{
			{
				name:            "usecaseがprocessed=trueを返すとき、applierに委譲して成功になる",
				payload:         validPayload,
				returnProcessed: true,
				wantErr:         false,
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.True(t, a.called, "applier に委譲する")
					assert.Equal(t, apiscenario.EventTypeOnboardingNameSet, a.gotType)
					assert.Equal(t, "p-1", a.gotPlayer)
					assert.Equal(t, validEventID, a.gotEvent)
					assert.Equal(t, "Kenya", a.gotName)
				},
			},
			{
				name:            "usecaseがprocessed=falseを返すとき、副作用なしで成功になる",
				payload:         validPayload,
				returnProcessed: false,
				wantErr:         false,
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.True(t, a.called, "冪等スキップも applier を経由して判定される")
				},
			},
			{
				name:            "不正なJSONのとき、applierに到達せず失敗になる",
				payload:         []byte("broken"),
				wantErr:         true,
				wantErrContains: "onboarding-name-set: bad payload",
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.False(t, a.called, "JSON parse 失敗時は applier に到達しない")
				},
			},
			{
				name: "未知のevent_typeのとき、applierに到達せず責務外として成功になる",
				payload: mustMarshal(t, apiscenario.OnboardingNameSetEvent{
					EventType: "unknown",
					EventID:   "22222222-2222-2222-2222-222222222222",
					PlayerID:  "p-2",
					Name:      "Kenya",
				}),
				wantErr: false,
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.False(t, a.called, "event_type フィルタで applier に到達しない")
				},
			},
			{
				name: "player_idが欠落するとき、applierに到達せず失敗になる",
				payload: mustMarshal(t, apiscenario.OnboardingNameSetEvent{
					EventType: apiscenario.EventTypeOnboardingNameSet,
					EventID:   "33333333-3333-3333-3333-333333333333",
					Name:      "Kenya",
				}),
				wantErr:         true,
				wantErrContains: "onboarding-name-set: missing required field",
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
				},
			},
			{
				name: "nameが欠落するとき、applierに到達せず失敗になる",
				payload: mustMarshal(t, apiscenario.OnboardingNameSetEvent{
					EventType: apiscenario.EventTypeOnboardingNameSet,
					EventID:   "44444444-4444-4444-4444-444444444444",
					PlayerID:  "p-3",
				}),
				wantErr:         true,
				wantErrContains: "onboarding-name-set: missing required field",
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
				},
			},
			{
				name:            "usecaseが汎用エラーを返すとき、失敗になる",
				payload:         validPayload,
				returnErr:       errors.New("db error"),
				wantErr:         true,
				wantErrContains: "onboarding-name-set: apply:",
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.True(t, a.called)
				},
			},
			{
				name:            "対象プレイヤーが存在せずusecaseがErrNotFoundを返すとき、publisherバグとして失敗になる",
				payload:         validPayload,
				returnErr:       port.ErrNotFound,
				wantErr:         true,
				wantErrContains: "onboarding-name-set: publisher bug:",
				assertApplier: func(t *testing.T, a *fakeOnboardingNameSetApplier) {
					assert.True(t, a.called)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				applier := &fakeOnboardingNameSetApplier{
					returnProcessed: tt.returnProcessed,
					returnErr:       tt.returnErr,
				}
				sub := NewOnboardingNameSetSubscriber(applier)

				err := sub.HandleMessage(context.Background(), tt.payload)

				assert.Equal(t, tt.wantErr, err != nil, "エラー有無 (err=%v)", err)
				assert.Contains(t, fmt.Sprintf("%v", err), tt.wantErrContains, "エラー内容が原因を区別できる")
				tt.assertApplier(t, applier)
			})
		}
	})
}
