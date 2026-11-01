package pubsubpush

import (
	"context"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
)

// OnboardingCompletedApplier はオンボーディング完了イベントを反映する。
type OnboardingCompletedApplier interface {
	ApplyCompleted(ctx context.Context, eventID, eventType, playerID string) error
}

// PlayerOnboardedSubscriber はオンボーディング完了イベントを購読する。
type PlayerOnboardedSubscriber struct {
	applier OnboardingCompletedApplier
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成する。
func NewPlayerOnboardedSubscriber(applier OnboardingCompletedApplier) *PlayerOnboardedSubscriber {
	return &PlayerOnboardedSubscriber{applier: applier}
}

// ReceivePlayerOnboarded はオンボーディング完了イベントを1件受け取り、プレイヤーの進行状態に反映する。
func (s *PlayerOnboardedSubscriber) ReceivePlayerOnboarded(ctx context.Context, event apiscenario.PlayerOnboardedEvent) error {
	if err := verifyEventType(ctx, PlayerOnboardedSubscription, event.EventType, apiscenario.EventTypePlayerOnboarded, event.EventID); err != nil {
		return err
	}
	ids, err := parseEventIDs(event.EventID, event.PlayerID)
	if err != nil {
		return reportFailure(ctx, PlayerOnboardedSubscription, failureMissingRequiredField, err, event.EventID, event.PlayerID)
	}
	if err := s.applier.ApplyCompleted(ctx, ids.eventID, event.EventType, ids.playerID); err != nil {
		return reportApplyFailure(ctx, PlayerOnboardedSubscription, err, event.EventID, event.PlayerID)
	}
	return nil
}
