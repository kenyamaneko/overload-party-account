package pubsub

import (
	"context"
	"errors"
	"testing"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// fakeOnboardingCompletedApplier は OnboardingCompletedApplier を満たす最小スタブ。
// subscriber は「usecase 層にイベント内容を正しく委譲し、戻り値に応じて
// 成功 / 失敗 / 警告ログ分岐だけを行う」契約なので、usecase 内部の Tx / repo
// 挙動はここで抽象化する。
type fakeOnboardingCompletedApplier struct {
	returnProcessed bool
	returnErr       error

	called    bool
	gotEvent  string
	gotType   string
	gotPlayer string
}

func (f *fakeOnboardingCompletedApplier) ApplyCompleted(
	_ context.Context,
	eventID, eventType, playerID string,
) (bool, error) {
	f.called = true
	f.gotEvent = eventID
	f.gotType = eventType
	f.gotPlayer = playerID
	if f.returnErr != nil {
		return false, f.returnErr
	}
	return f.returnProcessed, nil
}

func TestHandleMessagePlayerOnboarded(t *testing.T) {
	t.Run("player_onboarded イベントの処理", func(t *testing.T) {
		const validEventID = "11111111-1111-1111-1111-111111111111"
		validPayload := mustMarshal(t, apiscenario.PlayerOnboardedEvent{
			EventType:        apiscenario.EventTypePlayerOnboarded,
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
			assertApplier   func(t *testing.T, a *fakeOnboardingCompletedApplier)
		}{
			{
				name:            "usecase が processed=true を返すとき、applier に委譲して成功になる",
				payload:         validPayload,
				returnProcessed: true,
				wantErr:         false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called, "applier に委譲する")
					assert.Equal(t, apiscenario.EventTypePlayerOnboarded, a.gotType)
					assert.Equal(t, "p-1", a.gotPlayer)
					assert.Equal(t, validEventID, a.gotEvent)
				},
			},
			{
				name:            "usecase が processed=false を返すとき、副作用なしで成功になる",
				payload:         validPayload,
				returnProcessed: false,
				wantErr:         false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called, "冪等スキップも applier を経由して判定される")
				},
			},
			{
				name:    "不正な JSON のとき、applier に到達せず失敗になる",
				payload: []byte("broken"),
				wantErr: true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.False(t, a.called, "JSON parse 失敗時は applier に到達しない")
				},
			},
			{
				name: "未知の event_type のとき、applier に到達せず責務外として成功になる",
				payload: mustMarshal(t, apiscenario.PlayerOnboardedEvent{
					EventType: "unknown",
					EventID:   "22222222-2222-2222-2222-222222222222",
					PlayerID:  "p-2",
				}),
				wantErr: false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.False(t, a.called, "event_type フィルタで applier に到達しない")
				},
			},
			{
				name: "player_id が欠落するとき、applier に到達せず失敗になる",
				payload: mustMarshal(t, apiscenario.PlayerOnboardedEvent{
					EventType: apiscenario.EventTypePlayerOnboarded,
					EventID:   "33333333-3333-3333-3333-333333333333",
				}),
				wantErr: true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
				},
			},
			{
				name:      "usecase が汎用エラーを返すとき、失敗になる",
				payload:   validPayload,
				returnErr: errors.New("db error"),
				wantErr:   true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called)
				},
			},
			{
				name:      "usecase が ErrNotFound を返すとき、publisher バグとして失敗になる",
				payload:   validPayload,
				returnErr: port.ErrNotFound,
				wantErr:   true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				applier := &fakeOnboardingCompletedApplier{
					returnProcessed: tt.returnProcessed,
					returnErr:       tt.returnErr,
				}
				sub := NewPlayerOnboardedSubscriber(applier)

				err := sub.HandleMessage(context.Background(), tt.payload)

				assert.Equal(t, tt.wantErr, err != nil, "エラー有無 (err=%v)", err)
				tt.assertApplier(t, applier)
			})
		}
	})
}
