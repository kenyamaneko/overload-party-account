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

	"github.com/kenyamaneko/overload-party-account/internal/service"
)

// TestFactionPurchasedSubscriber_Consumes は「shop 購入起因の faction ロスター追加
// (selected_faction は変更しない) を 1 イベント単位で冪等に処理する」という仕様を
// Start() → stream.Consume → processEvent の経路で固定する。
//
// 契約検証は apishopfake 経由で shop 側の publish 型をそのまま使う
// (shop が schema を変えたら account のテストが compile / 実行で破綻する
// ように設計し、乖離を CI で検知する)。
func TestFactionPurchasedSubscriber_Consumes(t *testing.T) {
	const existingEventID = "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name          string
		publish        func(ctx context.Context, pub *apishopfake.Publisher, broker *apishopfake.Broker)
		seedProcessed map[string]string
		addErr        error
		insertErr     error
		wantAck       bool
		assertRepos   func(t *testing.T, factionRepo *fakeFactionRepo, eventRepo *fakeProcessedEventRepo)
	}{
		{
			name: "正常系: player_factions に shop_purchase で INSERT して ACK",
			publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
				_ = apishopfake.PublishFactionPurchased(ctx, pub, apishop.FactionPurchasedEvent{
					PlayerID: "p-1",
					Faction:  "SHE",
				})
			},
			wantAck: true,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				require.Len(t, factionRepo.added, 1)
				assert.Equal(t, "p-1", factionRepo.added[0].PlayerID)
				assert.Equal(t, "SHE", factionRepo.added[0].Faction)
				assert.Equal(t, service.FactionSourceShopPurchase, factionRepo.added[0].Source)
			},
		},
		{
			name: "冪等: 同一 event_id が既に processed_events にあれば副作用なしで ACK",
			publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
				_ = apishopfake.PublishFactionPurchased(ctx, pub, apishop.FactionPurchasedEvent{
					EventID:  existingEventID,
					PlayerID: "p-2",
					Faction:  "Tenki",
				})
			},
			seedProcessed: map[string]string{existingEventID: apishop.EventTypeFactionPurchased},
			wantAck:       true,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "不正 JSON: 握りつぶさず NACK",
			publish: func(_ context.Context, _ *apishopfake.Publisher, broker *apishopfake.Broker) {
				broker.Publish(apishop.TopicFactionPurchased, []byte("{not-json"))
			},
			wantAck: false,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "未知の event_type: 責務外として ACK (副作用なし)",
			publish: func(_ context.Context, _ *apishopfake.Publisher, broker *apishopfake.Broker) {
				payload, _ := json.Marshal(apishop.FactionPurchasedEvent{
					EventType: "unknown",
					EventID:   "22222222-2222-2222-2222-222222222222",
					PlayerID:  "p-3",
					Faction:   "Sugar",
				})
				broker.Publish(apishop.TopicFactionPurchased, payload)
			},
			wantAck: true,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "processed_events INSERT 失敗: NACK でリトライ",
			publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
				_ = apishopfake.PublishFactionPurchased(ctx, pub, apishop.FactionPurchasedEvent{
					PlayerID: "p-4",
					Faction:  "Tuners",
				})
			},
			insertErr: errors.New("db unavailable"),
			wantAck:   false,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "player_factions INSERT 失敗: NACK でリトライ",
			publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
				_ = apishopfake.PublishFactionPurchased(ctx, pub, apishop.FactionPurchasedEvent{
					PlayerID: "p-5",
					Faction:  "SHE",
				})
			},
			addErr:  errors.New("fk violation"),
			wantAck: false,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added, "AddPlayerFaction 失敗時は副作用が残らない")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := apishopfake.NewBroker()
			pub := apishopfake.NewPublisher(broker)
			stream := apishopfake.NewStream(apishopfake.NewSubscriber(broker), apishop.TopicFactionPurchased)

			factionRepo := &fakeFactionRepo{addErr: tt.addErr}
			eventRepo := newFakeProcessedEventRepo()
			eventRepo.insertErr = tt.insertErr
			for k, v := range tt.seedProcessed {
				eventRepo.seen[k] = v
			}

			sub := NewFactionPurchasedSubscriber(stream, factionRepo, fakeTxRunner{}, eventRepo)

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

			tt.assertRepos(t, factionRepo, eventRepo)
		})
	}
}
