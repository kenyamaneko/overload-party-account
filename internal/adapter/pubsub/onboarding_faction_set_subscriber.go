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

// OnboardingFactionSetApplier は OnboardingInteractor.ApplyFactionSet に対する最小インターフェース。
type OnboardingFactionSetApplier interface {
	ApplyFactionSet(ctx context.Context, eventID, eventType, playerID, initialFactionID string) (processed bool, err error)
}

// OnboardingFactionSetSubscriber は onboarding-faction-set subscription を消費する。
type OnboardingFactionSetSubscriber struct {
	stream  port.MessageStream
	applier OnboardingFactionSetApplier
}

// NewOnboardingFactionSetSubscriber は OnboardingFactionSetSubscriber を生成する。
func NewOnboardingFactionSetSubscriber(stream port.MessageStream, applier OnboardingFactionSetApplier) *OnboardingFactionSetSubscriber {
	return &OnboardingFactionSetSubscriber{stream: stream, applier: applier}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックする。
func (s *OnboardingFactionSetSubscriber) Start(ctx context.Context) error {
	slog.Info("onboarding-faction-set subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

func (s *OnboardingFactionSetSubscriber) processEvent(ctx context.Context, data []byte) error {
	var event apiscenario.OnboardingFactionSetEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("onboarding-faction-set subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("onboarding-faction-set: bad payload: %w", err)
	}
	if event.EventType != apiscenario.EventTypeOnboardingFactionSet {
		slog.Warn("onboarding-faction-set subscriber: unknown event_type, acking",
			"event_type", event.EventType, "event_id", event.EventID)
		return nil
	}
	if event.PlayerID == "" || event.InitialFactionID == "" {
		slog.Error("onboarding-faction-set subscriber: missing required field (nack)",
			"event_id", event.EventID, "player_id", event.PlayerID, "faction_empty", event.InitialFactionID == "")
		return fmt.Errorf("onboarding-faction-set: missing required field")
	}

	processed, err := s.applier.ApplyFactionSet(ctx, event.EventID, event.EventType, event.PlayerID, event.InitialFactionID)
	if err != nil {
		if usecase.IsPublisherBug(err) {
			slog.Error("onboarding-faction-set subscriber: publisher bug",
				"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
			return fmt.Errorf("onboarding-faction-set: publisher bug: %w", err)
		}
		slog.Error("onboarding-faction-set subscriber: apply failed",
			"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
		return fmt.Errorf("onboarding-faction-set: apply: %w", err)
	}
	if !processed {
		return nil
	}
	return nil
}
