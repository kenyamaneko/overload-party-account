package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// OnboardingCompletedApplier は OnboardingInteractor.ApplyCompleted に対する最小インターフェース。
type OnboardingCompletedApplier interface {
	ApplyCompleted(ctx context.Context, eventID, eventType, playerID string) (processed bool, err error)
}

// PlayerOnboardedSubscriber は player-onboarded subscription を消費する。
type PlayerOnboardedSubscriber struct {
	stream  port.MessageStream
	applier OnboardingCompletedApplier
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成する。
func NewPlayerOnboardedSubscriber(stream port.MessageStream, applier OnboardingCompletedApplier) *PlayerOnboardedSubscriber {
	return &PlayerOnboardedSubscriber{stream: stream, applier: applier}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックする。
func (s *PlayerOnboardedSubscriber) Start(ctx context.Context) error {
	slog.Info("player-onboarded subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

func (s *PlayerOnboardedSubscriber) processEvent(ctx context.Context, data []byte) error {
	var event apiscenario.PlayerOnboardedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("player-onboarded subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("player-onboarded: bad payload: %w", err)
	}
	if event.EventType != apiscenario.EventTypePlayerOnboarded {
		slog.Warn("player-onboarded subscriber: unknown event_type, acking",
			"event_type", event.EventType, "event_id", event.EventID)
		return nil
	}
	if event.PlayerID == "" {
		slog.Error("player-onboarded subscriber: missing player_id (nack)",
			"event_id", event.EventID)
		return fmt.Errorf("player-onboarded: missing player_id")
	}

	processed, err := s.applier.ApplyCompleted(ctx, event.EventID, event.EventType, event.PlayerID)
	if err != nil {
		if usecase.IsPublisherBug(err) {
			slog.Error("player-onboarded subscriber: publisher bug",
				"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
			return fmt.Errorf("player-onboarded: publisher bug: %w", err)
		}
		slog.Error("player-onboarded subscriber: apply failed",
			"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
		return fmt.Errorf("player-onboarded: apply: %w", err)
	}
	if !processed {
		return nil
	}
	return nil
}
