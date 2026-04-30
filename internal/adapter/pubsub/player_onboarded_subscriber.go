package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/service"
)

// OnboardingCompletedApplier は OnboardingService.ApplyCompleted に対する最小インターフェース。
type OnboardingCompletedApplier interface {
	ApplyCompleted(ctx context.Context, eventID, eventType, playerID string) (processed bool, err error)
}

// PlayerOnboardedSubscriber は player-onboarded subscription を消費し、
// players.onboarding_status='completed' への遷移のみを反映する。
// initial faction の永続化 (player_factions の is_initial=TRUE 行) は
// onboarding-faction-set の先行配信で完了済みである前提。
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
	var ev apiscenario.PlayerOnboardedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("player-onboarded subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("player-onboarded: bad payload: %w", err)
	}
	if ev.EventType != apiscenario.EventTypePlayerOnboarded {
		slog.Warn("player-onboarded subscriber: unknown event_type, acking",
			"event_type", ev.EventType, "event_id", ev.EventID)
		return nil
	}
	if ev.PlayerID == "" {
		slog.Error("player-onboarded subscriber: missing player_id (nack)",
			"event_id", ev.EventID)
		return fmt.Errorf("player-onboarded: missing player_id")
	}

	processed, err := s.applier.ApplyCompleted(ctx, ev.EventID, ev.EventType, ev.PlayerID)
	if err != nil {
		if service.IsPublisherBug(err) {
			slog.Error("player-onboarded subscriber: player not found (publisher bug)",
				"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
			return fmt.Errorf("player-onboarded: player not found: %w", err)
		}
		slog.Error("player-onboarded subscriber: apply failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("player-onboarded: apply: %w", err)
	}
	if !processed {
		return nil
	}
	return nil
}
