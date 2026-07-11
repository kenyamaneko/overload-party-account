package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	"github.com/kenyamaneko/overload-party-scenario/packages/api-scenario/apiscenariofake"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// topicPlayerOnboarded は apiscenariofake が PublishPlayerOnboarded /
// ExpectPlayerOnboarded で内部的にハードコードしているルーティングキー。
// raw bytes を broker.Publish するケース (不正 JSON / 未知 event_type) と
// NewStream の subscribe topic を一致させる必要がある。
const topicPlayerOnboarded = "player-onboarded"

// fakeOnboardingCompletedApplier は OnboardingCompletedApplier を満たす最小スタブ。
// subscriber は「usecase 層にイベント内容を正しく委譲し、戻り値に応じて
// ACK / NACK / 警告ログ分岐だけを行う」契約なので、usecase 内部の Tx / repo
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

func TestConsumePlayerOnboarded(t *testing.T) {
	t.Run("player_onboarded イベントの購読", func(t *testing.T) {
		validEvent := apiscenario.PlayerOnboardedEvent{
			PlayerID:         "p-1",
			InitialFactionID: "SHE",
		}

		publishValid := func(ctx context.Context, pub *apiscenariofake.Publisher, _ *apiscenariofake.Broker) {
			_ = apiscenariofake.PublishPlayerOnboarded(ctx, pub, validEvent)
		}

		tests := []struct {
			name            string
			publish         func(ctx context.Context, pub *apiscenariofake.Publisher, broker *apiscenariofake.Broker)
			returnProcessed bool
			returnErr       error
			wantAck         bool
			assertApplier   func(t *testing.T, a *fakeOnboardingCompletedApplier)
		}{
			{
				name:            "usecase が processed=true を返すとき、applier に委譲して ACK になる",
				publish:         publishValid,
				returnProcessed: true,
				wantAck:         true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called, "applier に委譲する")
					assert.Equal(t, apiscenario.EventTypePlayerOnboarded, a.gotType)
					assert.Equal(t, "p-1", a.gotPlayer)
					assert.NotEmpty(t, a.gotEvent, "EventID は fake の auto-fill で埋まる")
				},
			},
			{
				name:            "usecase が processed=false を返すとき、副作用なしで ACK になる",
				publish:         publishValid,
				returnProcessed: false,
				wantAck:         true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called, "冪等スキップも applier を経由して判定される")
				},
			},
			{
				name: "不正な JSON のとき、applier に到達せず NACK になる",
				publish: func(_ context.Context, _ *apiscenariofake.Publisher, broker *apiscenariofake.Broker) {
					broker.Publish(topicPlayerOnboarded, []byte("broken"))
				},
				wantAck: false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.False(t, a.called, "JSON parse 失敗時は applier に到達しない")
				},
			},
			{
				name: "未知の event_type のとき、applier に到達せず責務外として ACK になる",
				publish: func(_ context.Context, _ *apiscenariofake.Publisher, broker *apiscenariofake.Broker) {
					broker.Publish(topicPlayerOnboarded, mustMarshal(t, apiscenario.PlayerOnboardedEvent{
						EventType: "unknown",
						EventID:   "22222222-2222-2222-2222-222222222222",
						PlayerID:  "p-2",
					}))
				},
				wantAck: true,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.False(t, a.called, "event_type フィルタで applier に到達しない")
				},
			},
			{
				name: "player_id が欠落するとき、applier に到達せず NACK になる",
				publish: func(_ context.Context, _ *apiscenariofake.Publisher, broker *apiscenariofake.Broker) {
					broker.Publish(topicPlayerOnboarded, mustMarshal(t, apiscenario.PlayerOnboardedEvent{
						EventType: apiscenario.EventTypePlayerOnboarded,
						EventID:   "33333333-3333-3333-3333-333333333333",
					}))
				},
				wantAck: false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
				},
			},
			{
				name:      "usecase が汎用エラーを返すとき、NACK になる",
				publish:   publishValid,
				returnErr: errors.New("db error"),
				wantAck:   false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called)
				},
			},
			{
				name:      "usecase が ErrNotFound を返すとき、publisher バグとして NACK になる",
				publish:   publishValid,
				returnErr: port.ErrNotFound,
				wantAck:   false,
				assertApplier: func(t *testing.T, a *fakeOnboardingCompletedApplier) {
					assert.True(t, a.called)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				broker := apiscenariofake.NewBroker()
				pub := apiscenariofake.NewPublisher(broker)
				stream := apiscenariofake.NewStream(apiscenariofake.NewSubscriber(broker), topicPlayerOnboarded)

				applier := &fakeOnboardingCompletedApplier{
					returnProcessed: tt.returnProcessed,
					returnErr:       tt.returnErr,
				}
				sub := NewPlayerOnboardedSubscriber(stream, applier)

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				started := make(chan struct{})
				go func() {
					close(started)
					_ = sub.Start(ctx)
				}()
				<-started

				tt.publish(ctx, pub, broker)

				handlerErr := stream.ExpectHandled(t, time.Second)
				assert.Equal(t, tt.wantAck, handlerErr == nil, "ack 判定 (nil=ack, err=%v)", handlerErr)

				tt.assertApplier(t, applier)
			})
		}
	})
}
