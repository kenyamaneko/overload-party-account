package pubsubpush

import (
	"context"
	"errors"
	"time"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
)

var (
	errMissingOccurredAt = errors.New("timestamp is missing")
	errMissingIsPremium  = errors.New("is_premium is missing")
)

// PremiumUpdatedEvent は契約のプレミアム状態更新イベントである。
// 契約では必須の is_premium が、イベントに無いか null のときは nil になる。
type PremiumUpdatedEvent struct {
	apishop.PremiumUpdatedEvent
	IsPremium *bool `json:"is_premium"`
}

// PremiumUpdatedApplier はプレミアム状態更新イベントを反映する。
type PremiumUpdatedApplier interface {
	ApplyPremiumUpdated(
		ctx context.Context,
		eventID, eventType, playerID string,
		isPremium bool,
		expiresAt *time.Time,
		occurredAt time.Time,
	) error
}

// PremiumUpdatedSubscriber はプレミアム状態更新イベントを購読する。
type PremiumUpdatedSubscriber struct {
	applier PremiumUpdatedApplier
}

// NewPremiumUpdatedSubscriber は PremiumUpdatedSubscriber を生成する。
func NewPremiumUpdatedSubscriber(applier PremiumUpdatedApplier) *PremiumUpdatedSubscriber {
	return &PremiumUpdatedSubscriber{applier: applier}
}

// ReceivePremiumUpdated はプレミアム状態更新イベントを1件受け取り、プレイヤーのプレミアム状態に反映する。
func (s *PremiumUpdatedSubscriber) ReceivePremiumUpdated(ctx context.Context, event PremiumUpdatedEvent) error {
	if err := verifyEventType(ctx, PremiumUpdatedSubscription, event.EventType, apishop.EventTypePremiumUpdated, event.EventID); err != nil {
		return err
	}
	ids, err := parseEventIDs(event.EventID, event.PlayerID)
	if err != nil {
		return reportFailure(ctx, PremiumUpdatedSubscription, failureHandlerFailed, err, event.EventID, event.PlayerID)
	}
	if event.Timestamp.IsZero() {
		return reportFailure(ctx, PremiumUpdatedSubscription, failureHandlerFailed, errMissingOccurredAt, event.EventID, event.PlayerID)
	}
	if event.IsPremium == nil {
		return reportFailure(ctx, PremiumUpdatedSubscription, failureHandlerFailed, errMissingIsPremium, event.EventID, event.PlayerID)
	}
	err = s.applier.ApplyPremiumUpdated(ctx, ids.eventID, event.EventType, ids.playerID,
		*event.IsPremium, event.PremiumExpiresAt, event.Timestamp)
	if err != nil {
		return reportFailure(ctx, PremiumUpdatedSubscription, failureHandlerFailed, err, event.EventID, event.PlayerID)
	}
	return nil
}
