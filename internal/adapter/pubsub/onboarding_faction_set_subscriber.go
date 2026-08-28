package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// OnboardingFactionSetApplier は OnboardingInteractor.ApplyFactionSet に対する最小インターフェース。
type OnboardingFactionSetApplier interface {
	ApplyFactionSet(ctx context.Context, eventID, eventType, playerID, initialFactionID string) (processed bool, err error)
}

// OnboardingFactionSetSubscriber は onboarding-faction-set イベントを消費する。
type OnboardingFactionSetSubscriber struct {
	applier OnboardingFactionSetApplier
}

// NewOnboardingFactionSetSubscriber は OnboardingFactionSetSubscriber を生成する。
func NewOnboardingFactionSetSubscriber(applier OnboardingFactionSetApplier) *OnboardingFactionSetSubscriber {
	return &OnboardingFactionSetSubscriber{applier: applier}
}

// HandleMessage は 1 件の onboarding-faction-set イベントを処理する。port.MessageHandler を満たし、
// push 受け口 (internal/handler/pubsubpush) から呼ばれる。
func (s *OnboardingFactionSetSubscriber) HandleMessage(ctx context.Context, data []byte) error {
	var event apiscenario.OnboardingFactionSetEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("onboarding-faction-set subscriber: bad payload", "error", err)
		return fmt.Errorf("onboarding-faction-set: bad payload: %w", err)
	}
	if event.EventType != apiscenario.EventTypeOnboardingFactionSet {
		slog.Error("onboarding-faction-set subscriber: unknown event_type",
			"event_type", event.EventType, "event_id", event.EventID)
		return fmt.Errorf("onboarding-faction-set: unknown event_type %q", event.EventType)
	}
	if event.PlayerID == "" || event.InitialFactionID == "" {
		slog.Error("onboarding-faction-set subscriber: missing required field",
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
