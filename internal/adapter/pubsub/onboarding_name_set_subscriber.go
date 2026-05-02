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
// adapter 層が usecase の具象に直接依存しないために port 風に切る。
type OnboardingNameSetApplier interface {
	ApplyNameSet(ctx context.Context, eventID, eventType, playerID, name string) (processed bool, err error)
}

// OnboardingNameSetSubscriber は onboarding-name-set subscription を消費し、
// players.name と players.onboarding_status='name_set' を 1 tx で反映する。
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
	var ev apiscenario.OnboardingNameSetEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("onboarding-name-set subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("onboarding-name-set: bad payload: %w", err)
	}
	if ev.EventType != apiscenario.EventTypeOnboardingNameSet {
		slog.Warn("onboarding-name-set subscriber: unknown event_type, acking",
			"event_type", ev.EventType, "event_id", ev.EventID)
		return nil
	}
	if ev.PlayerID == "" || ev.Name == "" {
		slog.Error("onboarding-name-set subscriber: missing required field (nack)",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "name_empty", ev.Name == "")
		return fmt.Errorf("onboarding-name-set: missing required field")
	}

	processed, err := s.applier.ApplyNameSet(ctx, ev.EventID, ev.EventType, ev.PlayerID, ev.Name)
	if err != nil {
		if usecase.IsPublisherBug(err) {
			slog.Error("onboarding-name-set subscriber: player not found (publisher bug)",
				"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
			return fmt.Errorf("onboarding-name-set: player not found: %w", err)
		}
		slog.Error("onboarding-name-set subscriber: apply failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("onboarding-name-set: apply: %w", err)
	}
	if !processed {
		// processed=false は冪等スキップ (event_id 重複)。副作用なしで ACK。
		return nil
	}
	return nil
}
