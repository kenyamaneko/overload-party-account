package pubsub_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
)

func TestPlayerOnboardedSubscriber_HandleMessage(t *testing.T) {
	t.Run("[オンボーディング完了購読]イベント処理", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewPlayerOnboardedSubscriber(&fakeApplier{})

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.ErrorContains(t, err, "bad payload")
		})

		t.Run("イベント種別が対象外(player_onboarded以外)のとき、unknown event_typeを含むエラーを返す", func(t *testing.T) {
			s := pubsub.NewPlayerOnboardedSubscriber(&fakeApplier{requireEmpty: true})
			event := apiscenario.PlayerOnboardedEvent{
				EventType:        "unrelated_event",
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "player-1",
				InitialFactionID: "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.ErrorContains(t, err, "unknown event_type")
		})

		t.Run("イベント種別が対象外(player_onboarded以外)のとき、オンボーディング完了処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewPlayerOnboardedSubscriber(applier)
			event := apiscenario.PlayerOnboardedEvent{
				EventType:        "unrelated_event",
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "player-1",
				InitialFactionID: "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			_ = s.HandleMessage(context.Background(), data)

			assert.Nil(t, applier.calledWith)
		})

		t.Run("イベントに含まれる対象プレイヤーのIDが空文字のとき、エラーを返し、オンボーディング完了処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewPlayerOnboardedSubscriber(applier)
			event := apiscenario.PlayerOnboardedEvent{
				EventType:        apiscenario.EventTypePlayerOnboarded,
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "",
				InitialFactionID: "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
			assert.Nil(t, applier.calledWith)
		})

		t.Run("オンボーディング完了処理がエラーを返したとき、エラーを返す", func(t *testing.T) {
			applier := &fakeApplier{err: errors.New("boom")}
			s := pubsub.NewPlayerOnboardedSubscriber(applier)
			event := apiscenario.PlayerOnboardedEvent{
				EventType:        apiscenario.EventTypePlayerOnboarded,
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "player-1",
				InitialFactionID: "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
		})

		processedResultTests := []struct {
			name      string
			processed bool
		}{
			{"オンボーディング完了処理が重複配信によりスキップされたことを示す結果を返したとき", false},
			{"オンボーディング完了処理が正常に完了したことを示す結果を返したとき", true},
		}
		for _, tt := range processedResultTests {
			t.Run(tt.name+"、エラーにならない", func(t *testing.T) {
				applier := &fakeApplier{processed: tt.processed}
				s := pubsub.NewPlayerOnboardedSubscriber(applier)
				event := apiscenario.PlayerOnboardedEvent{
					EventType:        apiscenario.EventTypePlayerOnboarded,
					EventID:          "evt-1",
					Timestamp:        time.Now(),
					PlayerID:         "player-1",
					InitialFactionID: "SHE",
				}
				data, err := json.Marshal(event)
				require.NoError(t, err)

				err = s.HandleMessage(context.Background(), data)

				require.NoError(t, err)
			})
		}
	})
}
