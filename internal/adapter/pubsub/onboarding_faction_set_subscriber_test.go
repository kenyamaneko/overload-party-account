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

func TestOnboardingFactionSetSubscriber_HandleMessage(t *testing.T) {
	t.Run("[オンボーディング初期ファクション設定購読]イベント処理", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewOnboardingFactionSetSubscriber(&fakeApplier{})

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.Error(t, err)
		})

		t.Run("イベント種別が対象外(onboarding_faction_set以外)のとき、エラーにならず、オンボーディングの初期ファクション設定処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewOnboardingFactionSetSubscriber(applier)
			event := apiscenario.OnboardingFactionSetEvent{
				EventType:        "unrelated_event",
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "player-1",
				InitialFactionID: "SHE",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.Nil(t, applier.calledWith)
		})

		t.Run("イベントに含まれる対象プレイヤーのIDが空文字のとき、エラーを返し、オンボーディングの初期ファクション設定処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewOnboardingFactionSetSubscriber(applier)
			event := apiscenario.OnboardingFactionSetEvent{
				EventType:        apiscenario.EventTypeOnboardingFactionSet,
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

		t.Run("初期選択ファクションIDが空のとき、エラーを返し、オンボーディングの初期ファクション設定処理は実行されない", func(t *testing.T) {
			applier := &fakeApplier{requireEmpty: true}
			s := pubsub.NewOnboardingFactionSetSubscriber(applier)
			event := apiscenario.OnboardingFactionSetEvent{
				EventType:        apiscenario.EventTypeOnboardingFactionSet,
				EventID:          "evt-1",
				Timestamp:        time.Now(),
				PlayerID:         "player-1",
				InitialFactionID: "",
			}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			err = s.HandleMessage(context.Background(), data)

			require.Error(t, err)
			assert.Nil(t, applier.calledWith)
		})

		t.Run("オンボーディングの初期ファクション設定処理がエラーを返したとき、エラーを返す", func(t *testing.T) {
			applier := &fakeApplier{err: errors.New("boom")}
			s := pubsub.NewOnboardingFactionSetSubscriber(applier)
			event := apiscenario.OnboardingFactionSetEvent{
				EventType:        apiscenario.EventTypeOnboardingFactionSet,
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
			{"オンボーディングの初期ファクション設定処理が重複配信によりスキップされたことを示す結果を返したとき", false},
			{"オンボーディングの初期ファクション設定処理が正常に完了したことを示す結果を返したとき", true},
		}
		for _, tt := range processedResultTests {
			t.Run(tt.name+"、エラーにならない", func(t *testing.T) {
				applier := &fakeApplier{processed: tt.processed}
				s := pubsub.NewOnboardingFactionSetSubscriber(applier)
				event := apiscenario.OnboardingFactionSetEvent{
					EventType:        apiscenario.EventTypeOnboardingFactionSet,
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
