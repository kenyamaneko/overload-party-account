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

// TestConsumes_FactionAcquired は「shop 由来 (購入 / 配布) の faction ロスター追加
// (is_initial=FALSE 固定) を 1 イベント単位で冪等に処理する」という仕様を
// Start() → stream.Consume → processEvent の経路で固定する。
//
// 契約検証は apishop の wire 定数 (TopicFactionAcquired / EventTypeFactionAcquired /
// FactionAcquiredEvent) のみに依存させる — shop が schema を変えたら account の
// テストが compile / 実行で破綻するため、CI で乖離を検知できる。
func TestConsumes_FactionAcquired(t *testing.T) {
	const existingEventID = "11111111-1111-1111-1111-111111111111"

	publishEvent := func(broker *apishopfake.Broker, ev apishop.FactionAcquiredEvent) {
		if ev.EventType == "" {
			ev.EventType = apishop.EventTypeFactionAcquired
		}
		payload, err := json.Marshal(ev)
		require.NoError(t, err)
		broker.Publish(apishop.TopicFactionAcquired, payload)
	}

	tests := []struct {
		name          string
		publish       func(broker *apishopfake.Broker)
		seedProcessed map[string]string
		addErr        error
		insertErr     error
		wantAck       bool
		assertRepos   func(t *testing.T, factionRepo *fakeFactionRepo, eventRepo *fakeProcessedEventRepo)
	}{
		{
			name: "正常系: player_factions に shop_purchase で INSERT して ACK",
			publish: func(broker *apishopfake.Broker) {
				publishEvent(broker, apishop.FactionAcquiredEvent{
					EventID:  "33333333-3333-3333-3333-333333333333",
					PlayerID: "p-1",
					Faction:  "SHE",
				})
			},
			wantAck: true,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				require.Len(t, factionRepo.added, 1)
				assert.Equal(t, "p-1", factionRepo.added[0].PlayerID)
				assert.Equal(t, "SHE", factionRepo.added[0].Faction)
				assert.False(t, factionRepo.added[0].IsInitial)
			},
		},
		{
			name: "冪等: 同一 event_id が既に processed_events にあれば副作用なしで ACK",
			publish: func(broker *apishopfake.Broker) {
				publishEvent(broker, apishop.FactionAcquiredEvent{
					EventID:  existingEventID,
					PlayerID: "p-2",
					Faction:  "Tenki",
				})
			},
			seedProcessed: map[string]string{existingEventID: apishop.EventTypeFactionAcquired},
			wantAck:       true,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "不正 JSON: 握りつぶさず NACK",
			publish: func(broker *apishopfake.Broker) {
				broker.Publish(apishop.TopicFactionAcquired, []byte("{not-json"))
			},
			wantAck: false,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "未知の event_type: 責務外として ACK (副作用なし)",
			publish: func(broker *apishopfake.Broker) {
				publishEvent(broker, apishop.FactionAcquiredEvent{
					EventType: "unknown",
					EventID:   "22222222-2222-2222-2222-222222222222",
					PlayerID:  "p-3",
					Faction:   "Sugar",
				})
			},
			wantAck: true,
			assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "processed_events INSERT 失敗: NACK でリトライ",
			publish: func(broker *apishopfake.Broker) {
				publishEvent(broker, apishop.FactionAcquiredEvent{
					EventID:  "44444444-4444-4444-4444-444444444444",
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
			publish: func(broker *apishopfake.Broker) {
				publishEvent(broker, apishop.FactionAcquiredEvent{
					EventID:  "55555555-5555-5555-5555-555555555555",
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
			stream := apishopfake.NewStream(apishopfake.NewSubscriber(broker), apishop.TopicFactionAcquired)

			factionRepo := &fakeFactionRepo{addErr: tt.addErr}
			eventRepo := newFakeProcessedEventRepo()
			eventRepo.insertErr = tt.insertErr
			for k, v := range tt.seedProcessed {
				eventRepo.seen[k] = v
			}

			sub := NewFactionAcquiredSubscriber(stream, factionRepo, fakeTxRunner{}, eventRepo)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			started := make(chan struct{})
			go func() {
				close(started)
				_ = sub.Start(ctx)
			}()
			<-started

			tt.publish(broker)

			handlerErr := stream.ExpectHandled(t, time.Second)
			assert.Equal(t, tt.wantAck, handlerErr == nil, "ack 判定 (nil=ack, err=%v)", handlerErr)

			tt.assertRepos(t, factionRepo, eventRepo)
		})
	}
}
