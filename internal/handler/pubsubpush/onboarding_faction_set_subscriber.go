package pubsubpush

import (
	"context"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
)

// OnboardingFactionSetApplier はオンボーディング陣営設定イベントを反映する。
type OnboardingFactionSetApplier interface {
	ApplyFactionSet(ctx context.Context, eventID, eventType, playerID, initialFactionID string) error
}

// OnboardingFactionSetSubscriber はオンボーディング陣営設定イベントを購読する。
type OnboardingFactionSetSubscriber struct {
	applier OnboardingFactionSetApplier
}

// NewOnboardingFactionSetSubscriber は OnboardingFactionSetSubscriber を生成する。
func NewOnboardingFactionSetSubscriber(applier OnboardingFactionSetApplier) *OnboardingFactionSetSubscriber {
	return &OnboardingFactionSetSubscriber{applier: applier}
}

// ReceiveOnboardingFactionSet はオンボーディング陣営設定イベントを1件受け取り、プレイヤーの初期陣営に反映する。
func (s *OnboardingFactionSetSubscriber) ReceiveOnboardingFactionSet(ctx context.Context, event apiscenario.OnboardingFactionSetEvent) error {
	if err := verifyEventType(ctx, OnboardingFactionSetSubscription, event.EventType, apiscenario.EventTypeOnboardingFactionSet, event.EventID); err != nil {
		return err
	}
	ids, err := parseEventIDs(event.EventID, event.PlayerID)
	if err == nil {
		err = verifyRequiredField("initial_faction_id", event.InitialFactionID)
	}
	if err != nil {
		return reportFailure(ctx, OnboardingFactionSetSubscription, failureMissingRequiredField, err, event.EventID, event.PlayerID)
	}
	if err := s.applier.ApplyFactionSet(ctx, ids.eventID, event.EventType, ids.playerID, event.InitialFactionID); err != nil {
		return reportApplyFailure(ctx, OnboardingFactionSetSubscription, err, event.EventID, event.PlayerID)
	}
	return nil
}
