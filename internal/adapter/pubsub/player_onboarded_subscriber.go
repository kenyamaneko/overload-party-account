package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// OnboardingCompletedApplier は OnboardingInteractor.ApplyCompleted に対する最小インターフェース。
type OnboardingCompletedApplier interface {
	ApplyCompleted(ctx context.Context, eventID, eventType, playerID string) (processed bool, err error)
}

// PlayerOnboardedSubscriber は player-onboarded イベントを消費する。
type PlayerOnboardedSubscriber struct {
	applier OnboardingCompletedApplier
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成する。
func NewPlayerOnboardedSubscriber(applier OnboardingCompletedApplier) *PlayerOnboardedSubscriber {
	return &PlayerOnboardedSubscriber{applier: applier}
}

// HandleMessage は 1 件の player-onboarded イベントを処理する。port.MessageHandler を満たし、
// push 受け口 (internal/handler/pubsubpush) から呼ばれる。
func (s *PlayerOnboardedSubscriber) HandleMessage(ctx context.Context, data []byte) error {
	var event apiscenario.PlayerOnboardedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("player-onboarded subscriber: bad payload", "error", err)
		return fmt.Errorf("player-onboarded: bad payload: %w", err)
	}
	if event.EventType != apiscenario.EventTypePlayerOnboarded {
		slog.Error("player-onboarded subscriber: unknown event_type",
			"event_type", event.EventType, "event_id", event.EventID)
		return fmt.Errorf("player-onboarded: unknown event_type %q", event.EventType)
	}
	if event.PlayerID == "" {
		slog.Error("player-onboarded subscriber: missing player_id",
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
