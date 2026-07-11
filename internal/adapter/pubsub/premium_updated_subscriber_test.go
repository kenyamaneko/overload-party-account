package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
	"github.com/kenyamaneko/overload-party-shop/packages/api-shop/apishopfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// topicPremiumUpdated は apishopfake が PublishPremiumUpdated / ExpectPremiumUpdated で
// 内部的にハードコードしているルーティングキー。raw bytes を broker.Publish するケース
// (不正 JSON / 未知 event_type) と NewStream の subscribe topic を一致させる必要がある。
const topicPremiumUpdated = "premium-updated"

func TestConsumePremiumUpdated(t *testing.T) {
	t.Run("premium_updated イベントの購読", func(t *testing.T) {
		const existingEventID = "aaaaaaaa-1111-1111-1111-aaaaaaaaaaaa"
		expiry := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

		// 契約検証は apishopfake 経由で shop 側の publish 型をそのまま使う。shop が schema を
		// 変えたら本テストが compile / 実行で破綻し、乖離を CI で検知できるようにするため。
		tests := []struct {
			name          string
			publish       func(ctx context.Context, pub *apishopfake.Publisher, broker *apishopfake.Broker)
			seedProcessed map[string]string
			updateErr     error
			insertErr     error
			wantAck       bool
			assertRepos   func(t *testing.T, premiumRepo *fakePremiumRepo)
		}{
			{
				name: "premium=true + expiry を受けたとき、players に反映して ACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishPremiumUpdated(ctx, pub, apishop.PremiumUpdatedEvent{
						PlayerID:         "p-1",
						IsPremium:        true,
						PremiumExpiresAt: &expiry,
					})
				},
				wantAck: true,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					state, ok := premiumRepo.premium["p-1"]
					require.True(t, ok, "p-1 の premium 状態が反映されている")
					assert.True(t, state.IsPremium)
					require.NotNil(t, state.ExpiresAt)
					assert.True(t, state.ExpiresAt.Equal(expiry))
				},
			},
			{
				name: "premium=false を受けたとき、players に反映して ACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishPremiumUpdated(ctx, pub, apishop.PremiumUpdatedEvent{
						PlayerID:  "p-2",
						IsPremium: false,
					})
				},
				wantAck: true,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					state, ok := premiumRepo.premium["p-2"]
					require.True(t, ok)
					assert.False(t, state.IsPremium)
					assert.Nil(t, state.ExpiresAt)
				},
			},
			{
				name: "同一 event_id が processed_events に既にあるとき、副作用なしで ACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishPremiumUpdated(ctx, pub, apishop.PremiumUpdatedEvent{
						EventID:   existingEventID,
						PlayerID:  "p-3",
						IsPremium: true,
					})
				},
				seedProcessed: map[string]string{existingEventID: apishop.EventTypePremiumUpdated},
				wantAck:       true,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					_, ok := premiumRepo.premium["p-3"]
					assert.False(t, ok, "冪等スキップ時は player 状態が更新されない")
				},
			},
			{
				name: "不正な JSON のとき、握りつぶさず NACK になる",
				publish: func(_ context.Context, _ *apishopfake.Publisher, broker *apishopfake.Broker) {
					broker.Publish(topicPremiumUpdated, []byte("{malformed"))
				},
				wantAck: false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					assert.Empty(t, premiumRepo.premium, "JSON parse 失敗時は副作用が残らない")
				},
			},
			{
				name: "未知の event_type のとき、責務外として副作用なく ACK になる",
				publish: func(_ context.Context, _ *apishopfake.Publisher, broker *apishopfake.Broker) {
					payload, _ := json.Marshal(apishop.PremiumUpdatedEvent{
						EventType: "unknown",
						EventID:   "bbbbbbbb-2222-2222-2222-bbbbbbbbbbbb",
						PlayerID:  "p-4",
					})
					broker.Publish(topicPremiumUpdated, payload)
				},
				wantAck: true,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					_, ok := premiumRepo.premium["p-4"]
					assert.False(t, ok)
				},
			},
			{
				name: "processed_events への INSERT が失敗するとき、UpdatePremium まで到達せず NACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishPremiumUpdated(ctx, pub, apishop.PremiumUpdatedEvent{
						PlayerID:  "p-5",
						IsPremium: true,
					})
				},
				insertErr: errors.New("db unavailable"),
				wantAck:   false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					assert.Empty(t, premiumRepo.premium, "processed_events INSERT 失敗時は UpdatePremium まで到達しない")
				},
			},
			{
				name: "UpdatePremium が失敗するとき、tx rollback で副作用を残さず NACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishPremiumUpdated(ctx, pub, apishop.PremiumUpdatedEvent{
						PlayerID:  "p-6",
						IsPremium: true,
					})
				},
				updateErr: errors.New("row not found"),
				wantAck:   false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					assert.Empty(t, premiumRepo.premium, "UpdatePremium 失敗時は tx rollback により副作用が残らない")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				broker := apishopfake.NewBroker()
				pub := apishopfake.NewPublisher(broker)
				stream := apishopfake.NewStream(apishopfake.NewSubscriber(broker), topicPremiumUpdated)

				premiumRepo := newFakePremiumRepo()
				premiumRepo.updatePremiumErr = tt.updateErr
				eventRepo := newFakeProcessedEventRepo()
				eventRepo.insertErr = tt.insertErr
				for k, v := range tt.seedProcessed {
					eventRepo.seen[k] = v
				}

				sub := NewPremiumUpdatedSubscriber(stream, premiumRepo, fakeTxRunner{}, eventRepo)

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

				tt.assertRepos(t, premiumRepo)
			})
		}
	})
}
