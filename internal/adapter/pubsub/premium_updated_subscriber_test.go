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

func TestPremiumUpdatedSubscriber_HandleMessage(t *testing.T) {
	t.Run("PremiumUpdatedSubscriber", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewPremiumUpdatedSubscriber(newFakePlayerPremiumRepo(), fakeTxRunner{}, newFakeProcessedEventRepo())

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.Error(t, err)
		})

		t.Run("event_typeが一致しないとき、エラーを返さずに成功として処理を終える", func(t *testing.T) {
			premiumRepo := newFakePlayerPremiumRepo()
			s := pubsub.NewPremiumUpdatedSubscriber(premiumRepo, fakeTxRunner{}, newFakeProcessedEventRepo())
			event := apishop.PremiumUpdatedEvent{
				EventType: "unrelated_event",
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				IsPremium: true,
				Source:    apishop.PremiumUpdatedSourceShop,
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.False(t, premiumRepo.isPremium["player-1"])
		})

		t.Run("event_typeが一致し、同一event_idが未処理のとき、対象プレイヤーのプレミアムステータスと有効期限をイベントに含まれる値に更新する", func(t *testing.T) {
			premiumRepo := newFakePlayerPremiumRepo()
			s := pubsub.NewPremiumUpdatedSubscriber(premiumRepo, fakeTxRunner{}, newFakeProcessedEventRepo())
			expiresAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
			event := apishop.PremiumUpdatedEvent{
				EventType:        apishop.EventTypePremiumUpdated,
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "player-1",
				IsPremium:        true,
				PremiumExpiresAt: &expiresAt,
				Source:           apishop.PremiumUpdatedSourceShop,
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.True(t, premiumRepo.isPremium["player-1"])
			assert.Equal(t, &expiresAt, premiumRepo.expiresAt["player-1"])
		})

		t.Run("同一event_idが処理済みのとき、プレミアムステータスの更新は行わず、成功として応答する", func(t *testing.T) {
			premiumRepo := newFakePlayerPremiumRepo()
			eventRepo := newFakeProcessedEventRepo()
			s := pubsub.NewPremiumUpdatedSubscriber(premiumRepo, fakeTxRunner{}, eventRepo)
			event := apishop.PremiumUpdatedEvent{
				EventType: apishop.EventTypePremiumUpdated,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				IsPremium: true,
				Source:    apishop.PremiumUpdatedSourceShop,
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)
			require.NoError(t, s.HandleMessage(context.Background(), data))

			secondEvent := event
			secondEvent.IsPremium = false
			secondData, err := json.Marshal(secondEvent)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), secondData)

			require.NoError(t, err)
			assert.True(t, premiumRepo.isPremium["player-1"])
		})

		t.Run("プレミアムステータスの更新が失敗したとき、エラーを返す", func(t *testing.T) {
			premiumRepo := newFakePlayerPremiumRepo()
			premiumRepo.updateErr = errors.New("update failed")
			s := pubsub.NewPremiumUpdatedSubscriber(premiumRepo, fakeTxRunner{}, newFakeProcessedEventRepo())
			event := apishop.PremiumUpdatedEvent{
				EventType: apishop.EventTypePremiumUpdated,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				IsPremium: true,
				Source:    apishop.PremiumUpdatedSourceShop,
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
		})

		t.Run("プレミアムステータスの更新が失敗したとき、event_idの処理済み記録もロールバックされ、同一event_idで再配信されると再度処理が実行される", func(t *testing.T) {
			premiumRepo := newFakePlayerPremiumRepo()
			premiumRepo.updateErr = errors.New("update failed")
			eventRepo := newFakeProcessedEventRepo()
			s := pubsub.NewPremiumUpdatedSubscriber(premiumRepo, fakeTxRunner{}, eventRepo)
			event := apishop.PremiumUpdatedEvent{
				EventType: apishop.EventTypePremiumUpdated,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				IsPremium: true,
				Source:    apishop.PremiumUpdatedSourceShop,
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)
			require.Error(t, s.HandleMessage(context.Background(), data))

			premiumRepo.updateErr = nil
			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.True(t, premiumRepo.isPremium["player-1"])
		})
	})
}
