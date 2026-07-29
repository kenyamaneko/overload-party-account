package pubsub

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// fakeOnboardingFactionSetApplier は OnboardingFactionSetApplier を満たす最小スタブ。
// subscriber は「usecase 層にイベント内容を正しく委譲し、戻り値に応じて
// 成功 / 失敗ログ分岐だけを行う」契約なので、usecase 内部の Tx / repo
// 挙動はここで抽象化する。
type fakeOnboardingFactionSetApplier struct {
	returnProcessed bool
	returnErr       error

	called     bool
	gotEvent   string
	gotType    string
	gotPlayer  string
	gotFaction string
}

func (f *fakeOnboardingFactionSetApplier) ApplyFactionSet(
	_ context.Context,
	eventID, eventType, playerID, initialFactionID string,
) (bool, error) {
	f.called = true
	f.gotEvent = eventID
	f.gotType = eventType
	f.gotPlayer = playerID
	f.gotFaction = initialFactionID
	if f.returnErr != nil {
		return false, f.returnErr
	}
	return f.returnProcessed, nil
}

func TestHandleMessageOnboardingFactionSet(t *testing.T) {
	t.Run("onboarding_faction_set イベントの処理", func(t *testing.T) {
		const validEventID = "11111111-1111-1111-1111-111111111111"
		validPayload := mustMarshal(t, apiscenario.OnboardingFactionSetEvent{
			EventType:        apiscenario.EventTypeOnboardingFactionSet,
			EventID:          validEventID,
			PlayerID:         "p-1",
			InitialFactionID: "SHE",
		})

		tests := []struct {
			name            string
			payload         []byte
			returnProcessed bool
			returnErr       error
			wantErr         bool
			wantErrContains string
			assertApplier   func(t *testing.T, a *fakeOnboardingFactionSetApplier)
		}{
			{
				name:            "usecase が processed=true を返すとき、applier に委譲して成功になる",
				payload:         validPayload,
				returnProcessed: true,
				wantErr:         false,
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.True(t, a.called, "applier に委譲する")
					assert.Equal(t, apiscenario.EventTypeOnboardingFactionSet, a.gotType)
					assert.Equal(t, "p-1", a.gotPlayer)
					assert.Equal(t, validEventID, a.gotEvent)
					assert.Equal(t, "SHE", a.gotFaction)
				},
			},
			{
				name:            "usecase が processed=false を返すとき、副作用なしで成功になる",
				payload:         validPayload,
				returnProcessed: false,
				wantErr:         false,
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.True(t, a.called, "冪等スキップも applier を経由して判定される")
				},
			},
			{
				name:            "不正な JSON のとき、applier に到達せず失敗になる",
				payload:         []byte("broken"),
				wantErr:         true,
				wantErrContains: "onboarding-faction-set: bad payload",
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.False(t, a.called, "JSON parse 失敗時は applier に到達しない")
				},
			},
			{
				name: "未知の event_type のとき、applier に到達せず責務外として成功になる",
				payload: mustMarshal(t, apiscenario.OnboardingFactionSetEvent{
					EventType:        "unknown",
					EventID:          "22222222-2222-2222-2222-222222222222",
					PlayerID:         "p-2",
					InitialFactionID: "SHE",
				}),
				wantErr: false,
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.False(t, a.called, "event_type フィルタで applier に到達しない")
				},
			},
			{
				name: "player_id が欠落するとき、applier に到達せず失敗になる",
				payload: mustMarshal(t, apiscenario.OnboardingFactionSetEvent{
					EventType:        apiscenario.EventTypeOnboardingFactionSet,
					EventID:          "33333333-3333-3333-3333-333333333333",
					InitialFactionID: "SHE",
				}),
				wantErr:         true,
				wantErrContains: "onboarding-faction-set: missing required field",
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
				},
			},
			{
				name: "initial_faction_id が欠落するとき、applier に到達せず失敗になる",
				payload: mustMarshal(t, apiscenario.OnboardingFactionSetEvent{
					EventType: apiscenario.EventTypeOnboardingFactionSet,
					EventID:   "44444444-4444-4444-4444-444444444444",
					PlayerID:  "p-3",
				}),
				wantErr:         true,
				wantErrContains: "onboarding-faction-set: missing required field",
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
				},
			},
			{
				name:            "usecase が汎用エラーを返すとき、失敗になる",
				payload:         validPayload,
				returnErr:       errors.New("db error"),
				wantErr:         true,
				wantErrContains: "onboarding-faction-set: apply:",
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.True(t, a.called)
				},
			},
			{
				name:            "既に別の初期ファクションが設定済みで usecase が ErrFactionConflict を返すとき、publisher バグとして失敗になる",
				payload:         validPayload,
				returnErr:       usecase.ErrFactionConflict,
				wantErr:         true,
				wantErrContains: "onboarding-faction-set: publisher bug:",
				assertApplier: func(t *testing.T, a *fakeOnboardingFactionSetApplier) {
					assert.True(t, a.called)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				applier := &fakeOnboardingFactionSetApplier{
					returnProcessed: tt.returnProcessed,
					returnErr:       tt.returnErr,
				}
				sub := NewOnboardingFactionSetSubscriber(applier)

				err := sub.HandleMessage(context.Background(), tt.payload)

				assert.Equal(t, tt.wantErr, err != nil, "エラー有無 (err=%v)", err)
				assert.Contains(t, fmt.Sprintf("%v", err), tt.wantErrContains, "エラー内容が原因を区別できる")
				tt.assertApplier(t, applier)
			})
		}
	})
}
