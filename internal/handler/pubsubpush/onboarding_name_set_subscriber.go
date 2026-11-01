package pubsubpush

import (
	"context"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
)

// OnboardingNameSetApplier はオンボーディング表示名設定イベントを反映する。
type OnboardingNameSetApplier interface {
	ApplyNameSet(ctx context.Context, eventID, eventType, playerID, name string) error
}

// OnboardingNameSetSubscriber はオンボーディング表示名設定イベントを購読する。
type OnboardingNameSetSubscriber struct {
	applier OnboardingNameSetApplier
}

// NewOnboardingNameSetSubscriber は OnboardingNameSetSubscriber を生成する。
func NewOnboardingNameSetSubscriber(applier OnboardingNameSetApplier) *OnboardingNameSetSubscriber {
	return &OnboardingNameSetSubscriber{applier: applier}
}

// ReceiveOnboardingNameSet はオンボーディング表示名設定イベントを1件受け取り、プレイヤーの表示名に反映する。
func (s *OnboardingNameSetSubscriber) ReceiveOnboardingNameSet(ctx context.Context, event apiscenario.OnboardingNameSetEvent) error {
	if err := verifyEventType(ctx, OnboardingNameSetSubscription, event.EventType, apiscenario.EventTypeOnboardingNameSet, event.EventID); err != nil {
		return err
	}
	ids, err := parseEventIDs(event.EventID, event.PlayerID)
	if err == nil {
		err = verifyRequiredField("name", event.Name)
	}
	if err != nil {
		return reportFailure(ctx, OnboardingNameSetSubscription, failureMissingRequiredField, err, event.EventID, event.PlayerID)
	}
	if err := s.applier.ApplyNameSet(ctx, ids.eventID, event.EventType, ids.playerID, event.Name); err != nil {
		return reportApplyFailure(ctx, OnboardingNameSetSubscription, err, event.EventID, event.PlayerID)
	}
	return nil
}
