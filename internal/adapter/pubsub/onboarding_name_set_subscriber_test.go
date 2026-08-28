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

func TestOnboardingNameSetSubscriber_HandleMessage(t *testing.T) {
	t.Run("[オンボーディング表示名設定購読]イベント処理", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewOnboardingNameSetSubscriber(&fakeApplier{})

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.ErrorContains(t, err, "bad payload")
		})

		t.Run("イベント種別が対象外(onboarding_name_set以外)のとき、unknown event_typeを含むエラーを返す", func(t *testing.T) {
			s := pubsub.NewOnboardingNameSetSubscriber(&fakeApplier{requireEmpty: true})
			event := apiscenario.OnboardingNameSetEvent{
				EventType: "unrelated_event",
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Name:      "プレイヤー",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.ErrorContains(t, err, "unknown event_type")
		})

		t.Run("イベント種別が対象外(onboarding_name_set以外)のとき、オンボーディングの名前設定処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewOnboardingNameSetSubscriber(applier)
			event := apiscenario.OnboardingNameSetEvent{
				EventType: "unrelated_event",
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Name:      "プレイヤー",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			_ = s.HandleMessage(context.Background(), data)

			assert.Nil(t, applier.calledWith)
		})

		t.Run("イベントに含まれる対象プレイヤーのIDが空文字のとき、エラーを返し、オンボーディングの名前設定処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewOnboardingNameSetSubscriber(applier)
			event := apiscenario.OnboardingNameSetEvent{
				EventType: apiscenario.EventTypeOnboardingNameSet,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "",
				Name:      "プレイヤー",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
			assert.Nil(t, applier.calledWith)
		})

		t.Run("イベントに含まれる表示名が空文字のとき、エラーを返し、オンボーディングの名前設定処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewOnboardingNameSetSubscriber(applier)
			event := apiscenario.OnboardingNameSetEvent{
				EventType: apiscenario.EventTypeOnboardingNameSet,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Name:      "",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
			assert.Nil(t, applier.calledWith)
		})

		t.Run("オンボーディングの名前設定処理がエラーを返したとき、エラーを返す", func(t *testing.T) {
			applier := &fakeApplier{err: errors.New("boom")}
			s := pubsub.NewOnboardingNameSetSubscriber(applier)
			event := apiscenario.OnboardingNameSetEvent{
				EventType: apiscenario.EventTypeOnboardingNameSet,
				EventID:   "evt-1",
				Timestamp: time.Now(),
				PlayerID:  "player-1",
				Name:      "プレイヤー",
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
			{"オンボーディングの名前設定処理が重複配信によりスキップされたことを示す結果を返したとき", false},
			{"オンボーディングの名前設定処理が正常に完了したことを示す結果を返したとき", true},
		}
		for _, tt := range processedResultTests {
			t.Run(tt.name+"、エラーにならない", func(t *testing.T) {
				applier := &fakeApplier{processed: tt.processed}
				s := pubsub.NewOnboardingNameSetSubscriber(applier)
				event := apiscenario.OnboardingNameSetEvent{
					EventType: apiscenario.EventTypeOnboardingNameSet,
					EventID:   "evt-1",
					Timestamp: time.Now(),
					PlayerID:  "player-1",
					Name:      "プレイヤー",
				}
				data, err := json.Marshal(event)
				require.NoError(t, err)

				err = s.HandleMessage(context.Background(), data)

				require.NoError(t, err)
			})
		}
	})
}
