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

// fakeOnboardingApplier は OnboardingApplier を満たす最小スタブ。
// subscriber は「service 層にイベント内容を正しく委譲し、戻り値に応じて
// ACK / NACK / 警告ログ分岐だけを行う」契約なので、service 内部の Tx / repo
// 挙動はここで抽象化する (subscriber テストで repo fake を直接触らない)。
type fakeOnboardingApplier struct {
	// returnProcessed / returnSelected は Apply の戻り値を制御する。
	returnProcessed bool
	returnSelected  bool
	// returnErr を設定すると Apply が常にこのエラーを返す。
	returnErr error

	called    bool
	gotEvent  string
	gotType   string
	gotPlayer string
	gotName   string
	gotFac    string
}

func (f *fakeOnboardingApplier) ApplyOnboardingResult(
	_ context.Context,
	eventID, eventType, playerID, displayName, initialFactionID string,
) (bool, bool, error) {
	f.called = true
	f.gotEvent = eventID
	f.gotType = eventType
	f.gotPlayer = playerID
	f.gotName = displayName
	f.gotFac = initialFactionID
	if f.returnErr != nil {
		return false, false, f.returnErr
	}
	return f.returnProcessed, f.returnSelected, nil
}

// TestPlayerOnboardedSubscriber_Consumes は「Pub/Sub ペイロードを Unmarshal して
// OnboardingApplier に委譲する」subscriber 単体の仕様を Start() → stream.Consume
// → processEvent の経路で固定する。scenario 側の契約検証は apiscenariofake 経由で
// scenario の publish 型をそのまま使う (scenario が schema を変えたら account
// のテストが compile / 実行で破綻するように設計し、乖離を CI で検知する)。
//
// SelectableFactions / 冪等ガード / Tx 境界などの業務不変条件は service 層の
// 責務なのでここではテストせず、返却値ごとの ACK / NACK / applier 呼び出し有無
// のみを確認する。
func TestPlayerOnboardedSubscriber_Consumes(t *testing.T) {
	validEvent := apiscenario.PlayerOnboardedEvent{
		PlayerID:         "p-1",
		DisplayName:      "name-1",
		InitialFactionID: "SHE",
	}

	publishValid := func(ctx context.Context, pub *apiscenariofake.Publisher, _ *apiscenariofake.Broker) {
		_ = apiscenariofake.PublishPlayerOnboarded(ctx, pub, validEvent)
	}

	tests := []struct {
		name            string
		publish         func(ctx context.Context, pub *apiscenariofake.Publisher, broker *apiscenariofake.Broker)
		returnProcessed bool
		returnSelected  bool
		returnErr       error
		wantAck         bool
		assertApplier   func(t *testing.T, a *fakeOnboardingApplier)
	}{
		{
			name:            "正常系: service が processed=true/selected=true を返したら ACK",
			publish:         publishValid,
			returnProcessed: true,
			returnSelected:  true,
			wantAck:         true,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.True(t, a.called, "applier に委譲する")
				assert.Equal(t, apiscenario.EventTypePlayerOnboarded, a.gotType)
				assert.Equal(t, "p-1", a.gotPlayer)
				assert.Equal(t, "name-1", a.gotName)
				assert.Equal(t, "SHE", a.gotFac)
				assert.NotEmpty(t, a.gotEvent, "EventID は fake の auto-fill で埋まる")
			},
		},
		{
			name:            "冪等スキップ: service が processed=false を返したら副作用なし ACK",
			publish:         publishValid,
			returnProcessed: false,
			wantAck:         true,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.True(t, a.called, "冪等スキップも applier を経由して判定される")
			},
		},
		{
			name:            "selected_faction 既存: processed=true/selected=false は警告ログのみで ACK",
			publish:         publishValid,
			returnProcessed: true,
			returnSelected:  false,
			wantAck:         true,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.True(t, a.called)
			},
		},
		{
			name: "不正 JSON: 握りつぶさず NACK (applier 未呼び出し)",
			publish: func(_ context.Context, _ *apiscenariofake.Publisher, broker *apiscenariofake.Broker) {
				broker.Publish(apiscenario.TopicPlayerOnboarded, []byte("broken"))
			},
			wantAck: false,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.False(t, a.called, "JSON parse 失敗時は applier に到達しない")
			},
		},
		{
			name: "未知 event_type: 責務外として ACK (applier 未呼び出し)",
			publish: func(_ context.Context, _ *apiscenariofake.Publisher, broker *apiscenariofake.Broker) {
				broker.Publish(apiscenario.TopicPlayerOnboarded, mustMarshal(t, apiscenario.PlayerOnboardedEvent{
					EventType: "unknown",
					EventID:   "22222222-2222-2222-2222-222222222222",
					PlayerID:  "p-2",
				}))
			},
			wantAck: true,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.False(t, a.called, "event_type フィルタで applier に到達しない")
			},
		},
		{
			name: "initial_faction_id 欠落: ペイロード仕様違反で NACK (applier 未呼び出し)",
			publish: func(_ context.Context, _ *apiscenariofake.Publisher, broker *apiscenariofake.Broker) {
				broker.Publish(apiscenario.TopicPlayerOnboarded, mustMarshal(t, apiscenario.PlayerOnboardedEvent{
					EventType:   apiscenario.EventTypePlayerOnboarded,
					EventID:     "33333333-3333-3333-3333-333333333333",
					PlayerID:    "p-3",
					DisplayName: "x",
				}))
			},
			wantAck: false,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.False(t, a.called, "必須フィールド欠落は applier より手前で弾く")
			},
		},
		{
			name:      "service 汎用エラー: NACK でリトライ",
			publish:   publishValid,
			returnErr: errors.New("db error"),
			wantAck:   false,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.True(t, a.called)
			},
		},
		{
			name:      "service が ErrNotFound: publisher バグとして明示 NACK (error ログ)",
			publish:   publishValid,
			returnErr: port.ErrNotFound,
			wantAck:   false,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.True(t, a.called)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := apiscenariofake.NewBroker()
			pub := apiscenariofake.NewPublisher(broker)
			stream := apiscenariofake.NewStream(apiscenariofake.NewSubscriber(broker), apiscenario.TopicPlayerOnboarded)

			applier := &fakeOnboardingApplier{
				returnProcessed: tt.returnProcessed,
				returnSelected:  tt.returnSelected,
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
}
