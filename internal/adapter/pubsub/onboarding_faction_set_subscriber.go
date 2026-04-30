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

// OnboardingFactionSetApplier は OnboardingService.ApplyFactionSet に対する最小インターフェース。
type OnboardingFactionSetApplier interface {
	ApplyFactionSet(ctx context.Context, eventID, eventType, playerID, initialFactionID string) (processed bool, err error)
}

// OnboardingFactionSetSubscriber は onboarding-faction-set subscription を消費し、
// player_factions の is_initial=TRUE 行 UPSERT + players.onboarding_status='faction_set'
// を 1 tx で反映する。
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
	var ev apiscenario.OnboardingFactionSetEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("onboarding-faction-set subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("onboarding-faction-set: bad payload: %w", err)
	}
	if ev.EventType != apiscenario.EventTypeOnboardingFactionSet {
		slog.Warn("onboarding-faction-set subscriber: unknown event_type, acking",
			"event_type", ev.EventType, "event_id", ev.EventID)
		return nil
	}
	if ev.PlayerID == "" || ev.InitialFactionID == "" {
		slog.Error("onboarding-faction-set subscriber: missing required field (nack)",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "faction_empty", ev.InitialFactionID == "")
		return fmt.Errorf("onboarding-faction-set: missing required field")
	}

	processed, err := s.applier.ApplyFactionSet(ctx, ev.EventID, ev.EventType, ev.PlayerID, ev.InitialFactionID)
	if err != nil {
		if service.IsPublisherBug(err) {
			slog.Error("onboarding-faction-set subscriber: player not found (publisher bug)",
				"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
			return fmt.Errorf("onboarding-faction-set: player not found: %w", err)
		}
		slog.Error("onboarding-faction-set subscriber: apply failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("onboarding-faction-set: apply: %w", err)
	}
	if !processed {
		return nil
	}
	return nil
}
