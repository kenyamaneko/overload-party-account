package pubsub_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
)

func TestFactionAcquiredSubscriber_HandleMessage(t *testing.T) {
	t.Run("FactionAcquiredSubscriber", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewFactionAcquiredSubscriber(newFakeFactionRepo(), fakeTxRunner{}, newFakeProcessedEventRepo())

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.Error(t, err)
		})

		t.Run("event_typeが一致しないとき、エラーを返さずに成功として処理を終える", func(t *testing.T) {
			factionRepo := newFakeFactionRepo()
			s := pubsub.NewFactionAcquiredSubscriber(factionRepo, fakeTxRunner{}, newFakeProcessedEventRepo())
			event := apishop.FactionAcquiredEvent{
				EventType: "unrelated_event",
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Faction:   "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.Empty(t, factionRepo.factions["player-1"])
		})

		t.Run("event_typeが一致し、同一event_idが未処理のとき、対象プレイヤーに指定ファクションを所持ファクションとして追加する", func(t *testing.T) {
			factionRepo := newFakeFactionRepo()
			s := pubsub.NewFactionAcquiredSubscriber(factionRepo, fakeTxRunner{}, newFakeProcessedEventRepo())
			event := apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Faction:   "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.Equal(t, []string{"SHE"}, factionRepo.factions["player-1"])
		})

		t.Run("同一event_idが処理済みのとき、ファクションの追加は行わず、成功として応答する", func(t *testing.T) {
			factionRepo := newFakeFactionRepo()
			eventRepo := newFakeProcessedEventRepo()
			s := pubsub.NewFactionAcquiredSubscriber(factionRepo, fakeTxRunner{}, eventRepo)
			event := apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Faction:   "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)
			require.NoError(t, s.HandleMessage(context.Background(), data))

			secondEvent := event
			secondEvent.Faction = "Tenki"
			secondData, err := json.Marshal(secondEvent)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), secondData)

			require.NoError(t, err)
			assert.Equal(t, []string{"SHE"}, factionRepo.factions["player-1"])
		})

		t.Run("ファクションの追加が失敗したとき、エラーを返す", func(t *testing.T) {
			factionRepo := newFakeFactionRepo()
			factionRepo.addErr = errors.New("insert failed")
			s := pubsub.NewFactionAcquiredSubscriber(factionRepo, fakeTxRunner{}, newFakeProcessedEventRepo())
			event := apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Faction:   "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
		})

		t.Run("ファクションの追加が失敗したとき、event_idの処理済み記録もロールバックされ、同一event_idで再配信されると再度処理が実行される", func(t *testing.T) {
			factionRepo := newFakeFactionRepo()
			factionRepo.addErr = errors.New("insert failed")
			eventRepo := newFakeProcessedEventRepo()
			s := pubsub.NewFactionAcquiredSubscriber(factionRepo, fakeTxRunner{}, eventRepo)
			event := apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Faction:   "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)
			require.Error(t, s.HandleMessage(context.Background(), data))

			factionRepo.addErr = nil
			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.Equal(t, []string{"SHE"}, factionRepo.factions["player-1"])
		})
	})
}
