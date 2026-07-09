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

// topicFactionAcquired は apishopfake が PublishFactionAcquired / ExpectFactionAcquired で
// 内部的にハードコードしているルーティングキー。raw bytes を broker.Publish するケース
// (不正 JSON / 未知 event_type) と NewStream の subscribe topic を一致させる必要がある。
const topicFactionAcquired = "faction-acquired"

func TestConsumeFactionAcquired(t *testing.T) {
	t.Run("faction_acquired イベントの購読", func(t *testing.T) {
		const existingEventID = "11111111-1111-1111-1111-111111111111"

		// 契約検証は apishopfake 経由で shop 側の publish 型をそのまま使う。shop が schema を
		// 変えたら本テストが compile / 実行で破綻し、乖離を CI で検知できるようにするため。
		tests := []struct {
			name          string
			publish       func(ctx context.Context, pub *apishopfake.Publisher, broker *apishopfake.Broker)
			seedProcessed map[string]string
			addErr        error
			insertErr     error
			wantAck       bool
			assertRepos   func(t *testing.T, factionRepo *fakeFactionRepo, eventRepo *fakeProcessedEventRepo)
		}{
			{
				name: "有効な faction_acquired を受けたとき、player_factions に is_initial=FALSE で追加して ACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishFactionAcquired(ctx, pub, apishop.FactionAcquiredEvent{
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
				name: "同一 event_id が processed_events に既にあるとき、副作用なしで ACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishFactionAcquired(ctx, pub, apishop.FactionAcquiredEvent{
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
				name: "不正な JSON のとき、握りつぶさず NACK になる",
				publish: func(_ context.Context, _ *apishopfake.Publisher, broker *apishopfake.Broker) {
					broker.Publish(topicFactionAcquired, []byte("{not-json"))
				},
				wantAck: false,
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added)
				},
			},
			{
				name: "未知の event_type のとき、責務外として副作用なく ACK になる",
				publish: func(_ context.Context, _ *apishopfake.Publisher, broker *apishopfake.Broker) {
					payload, _ := json.Marshal(apishop.FactionAcquiredEvent{
						EventType: "unknown",
						EventID:   "22222222-2222-2222-2222-222222222222",
						PlayerID:  "p-3",
						Faction:   "Sugar",
					})
					broker.Publish(topicFactionAcquired, payload)
				},
				wantAck: true,
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added)
				},
			},
			{
				name: "processed_events への INSERT が失敗するとき、NACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishFactionAcquired(ctx, pub, apishop.FactionAcquiredEvent{
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
				name: "player_factions への追加が失敗するとき、副作用を残さず NACK になる",
				publish: func(ctx context.Context, pub *apishopfake.Publisher, _ *apishopfake.Broker) {
					_ = apishopfake.PublishFactionAcquired(ctx, pub, apishop.FactionAcquiredEvent{
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
				stream := apishopfake.NewStream(apishopfake.NewSubscriber(broker), topicFactionAcquired)

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

				tt.publish(ctx, pub, broker)

				handlerErr := stream.ExpectHandled(t, time.Second)
				assert.Equal(t, tt.wantAck, handlerErr == nil, "ack 判定 (nil=ack, err=%v)", handlerErr)

				tt.assertRepos(t, factionRepo, eventRepo)
			})
		}
	})
}
