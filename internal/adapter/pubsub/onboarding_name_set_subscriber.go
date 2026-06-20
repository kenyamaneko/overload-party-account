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

// OnboardingNameSetApplier は OnboardingInteractor.ApplyNameSet に対する最小インターフェース。
type OnboardingNameSetApplier interface {
	ApplyNameSet(ctx context.Context, eventID, eventType, playerID, name string) (processed bool, err error)
}

// OnboardingNameSetSubscriber は onboarding-name-set subscription を消費する。
type OnboardingNameSetSubscriber struct {
	stream  port.MessageStream
	applier OnboardingNameSetApplier
}

// NewOnboardingNameSetSubscriber は OnboardingNameSetSubscriber を生成する。
func NewOnboardingNameSetSubscriber(stream port.MessageStream, applier OnboardingNameSetApplier) *OnboardingNameSetSubscriber {
	return &OnboardingNameSetSubscriber{stream: stream, applier: applier}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックする。
func (s *OnboardingNameSetSubscriber) Start(ctx context.Context) error {
	slog.Info("onboarding-name-set subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

func (s *OnboardingNameSetSubscriber) processEvent(ctx context.Context, data []byte) error {
	var event apiscenario.OnboardingNameSetEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("onboarding-name-set subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("onboarding-name-set: bad payload: %w", err)
	}
	if event.EventType != apiscenario.EventTypeOnboardingNameSet {
		slog.Warn("onboarding-name-set subscriber: unknown event_type, acking",
			"event_type", event.EventType, "event_id", event.EventID)
		return nil
	}
	if event.PlayerID == "" || event.Name == "" {
		slog.Error("onboarding-name-set subscriber: missing required field (nack)",
			"event_id", event.EventID, "player_id", event.PlayerID, "name_empty", event.Name == "")
		return fmt.Errorf("onboarding-name-set: missing required field")
	}

	processed, err := s.applier.ApplyNameSet(ctx, event.EventID, event.EventType, event.PlayerID, event.Name)
	if err != nil {
		if usecase.IsPublisherBug(err) {
			slog.Error("onboarding-name-set subscriber: publisher bug",
				"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
			return fmt.Errorf("onboarding-name-set: publisher bug: %w", err)
		}
		slog.Error("onboarding-name-set subscriber: apply failed",
			"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
		return fmt.Errorf("onboarding-name-set: apply: %w", err)
	}
	if !processed {
		return nil
	}
	return nil
}
