package pubsubpush

import (
	"context"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
)

// FactionAcquiredApplier は陣営獲得イベントを反映する。
type FactionAcquiredApplier interface {
	ApplyFactionAcquired(ctx context.Context, eventID, eventType, playerID, faction string) error
}

// FactionAcquiredSubscriber は陣営獲得イベントを購読する。
type FactionAcquiredSubscriber struct {
	applier FactionAcquiredApplier
}

// NewFactionAcquiredSubscriber は FactionAcquiredSubscriber を生成する。
func NewFactionAcquiredSubscriber(applier FactionAcquiredApplier) *FactionAcquiredSubscriber {
	return &FactionAcquiredSubscriber{applier: applier}
}

// ReceiveFactionAcquired は陣営獲得イベントを1件受け取り、所持陣営に反映する。
func (s *FactionAcquiredSubscriber) ReceiveFactionAcquired(ctx context.Context, event apishop.FactionAcquiredEvent) error {
	if err := verifyEventType(ctx, FactionAcquiredSubscription, event.EventType, apishop.EventTypeFactionAcquired, event.EventID); err != nil {
		return err
	}
	ids, err := parseEventIDs(event.EventID, event.PlayerID)
	if err != nil {
		return reportFailure(ctx, FactionAcquiredSubscription, failureHandlerFailed, err, event.EventID, event.PlayerID)
	}
	if err := s.applier.ApplyFactionAcquired(ctx, ids.eventID, event.EventType, ids.playerID, event.Faction); err != nil {
		return reportFailure(ctx, FactionAcquiredSubscription, failureHandlerFailed, err, event.EventID, event.PlayerID)
	}
	return nil
}
