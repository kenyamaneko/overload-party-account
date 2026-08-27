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
	t.Run("[プレミアム状態更新購読]イベント処理", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewPremiumUpdatedSubscriber(newFakePlayerPremiumRepo(), fakeTxRunner{}, newFakeProcessedEventRepo())

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.ErrorContains(t, err, "bad payload")
		})

		t.Run("イベント種別が対象外(premium_updated以外)のとき、unknown event_typeを含むエラーを返す", func(t *testing.T) {
			s := pubsub.NewPremiumUpdatedSubscriber(newFakePlayerPremiumRepo(), fakeTxRunner{}, newFakeProcessedEventRepo())
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

			require.ErrorContains(t, err, "unknown event_type")
		})

		t.Run("イベント種別が対象外(premium_updated以外)のとき、対象プレイヤーのプレミアムステータスは更新されない", func(t *testing.T) {
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

			_ = s.HandleMessage(context.Background(), data)

			assert.False(t, premiumRepo.isPremium["player-1"])
		})

		t.Run("イベント種別がpremium_updatedで、同一のイベントIDを処理するのが初めてのとき、対象プレイヤーのプレミアムステータスと有効期限を、イベントに含まれる値に更新する", func(t *testing.T) {
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

		t.Run("同一のイベントIDが処理済みのとき、プレミアムステータスの更新は行われず、エラーにならない", func(t *testing.T) {
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

		t.Run("プレミアムステータスの更新が失敗した後、同一のイベントIDでメッセージが再送されると、エラーにならず対象プレイヤーのプレミアムステータスが更新される", func(t *testing.T) {
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
