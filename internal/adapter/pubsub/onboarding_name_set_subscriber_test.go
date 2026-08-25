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
	t.Run("OnboardingNameSetSubscriber", func(t *testing.T) {
		t.Run("ペイロードがJSONとして解析できないとき、エラーを返す", func(t *testing.T) {
			s := pubsub.NewOnboardingNameSetSubscriber(&fakeApplier{})

			err := s.HandleMessage(context.Background(), []byte("not-json"))

			require.Error(t, err)
		})

		t.Run("event_typeが一致しないとき、エラーを返さずに処理を終える", func(t *testing.T) {
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

			err = s.HandleMessage(context.Background(), data)

			require.NoError(t, err)
			assert.Nil(t, applier.calledWith)
		})

		t.Run("player_idが空のとき、委譲せずにエラーを返す", func(t *testing.T) {
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
		})

		t.Run("nameが空のとき、委譲せずにエラーを返す", func(t *testing.T) {
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
		})

		t.Run("委譲先がエラーを返したとき、subscriberはエラーを返す", func(t *testing.T) {
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

		t.Run("委譲先が重複配信によりスキップされたことを示す結果を返したとき、成功として応答する", func(t *testing.T) {
			applier := &fakeApplier{processed: false}
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

		t.Run("委譲先が正常に完了したことを示す結果を返したとき、成功として応答する", func(t *testing.T) {
			applier := &fakeApplier{processed: true}
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
	})
}
