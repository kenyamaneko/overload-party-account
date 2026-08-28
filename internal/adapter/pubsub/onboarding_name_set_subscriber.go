package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// OnboardingNameSetApplier は OnboardingInteractor.ApplyNameSet に対する最小インターフェース。
type OnboardingNameSetApplier interface {
	ApplyNameSet(ctx context.Context, eventID, eventType, playerID, name string) (processed bool, err error)
}

// OnboardingNameSetSubscriber は onboarding-name-set イベントを消費する。
type OnboardingNameSetSubscriber struct {
	applier OnboardingNameSetApplier
}

// NewOnboardingNameSetSubscriber は OnboardingNameSetSubscriber を生成する。
func NewOnboardingNameSetSubscriber(applier OnboardingNameSetApplier) *OnboardingNameSetSubscriber {
	return &OnboardingNameSetSubscriber{applier: applier}
}

// HandleMessage は 1 件の onboarding-name-set イベントを処理する。port.MessageHandler を満たし、
// push 受け口 (internal/handler/pubsubpush) から呼ばれる。
func (s *OnboardingNameSetSubscriber) HandleMessage(ctx context.Context, data []byte) error {
	var event apiscenario.OnboardingNameSetEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("onboarding-name-set subscriber: bad payload", "error", err)
		return fmt.Errorf("onboarding-name-set: bad payload: %w", err)
	}
	if event.EventType != apiscenario.EventTypeOnboardingNameSet {
		slog.Error("onboarding-name-set subscriber: unknown event_type",
			"event_type", event.EventType, "event_id", event.EventID)
		return fmt.Errorf("onboarding-name-set: unknown event_type %q", event.EventType)
	}
	if event.PlayerID == "" || event.Name == "" {
		slog.Error("onboarding-name-set subscriber: missing required field",
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
