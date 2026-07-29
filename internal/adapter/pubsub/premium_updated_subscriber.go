package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PremiumUpdatedSubscriber は premium-updated イベントを消費する。
type PremiumUpdatedSubscriber struct {
	premiumRepo port.PlayerPremiumRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewPremiumUpdatedSubscriber は PremiumUpdatedSubscriber を生成する。
func NewPremiumUpdatedSubscriber(
	premiumRepo port.PlayerPremiumRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) *PremiumUpdatedSubscriber {
	return &PremiumUpdatedSubscriber{
		premiumRepo: premiumRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}
}

// HandleMessage は 1 件の premium-updated イベントを処理する。port.MessageHandler を満たし、
// push 受け口 (internal/handler/pubsubpush) から呼ばれる。
func (s *PremiumUpdatedSubscriber) HandleMessage(ctx context.Context, data []byte) error {
	var event apishop.PremiumUpdatedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("premium-updated subscriber: bad payload", "error", err)
		return fmt.Errorf("premium-updated: bad payload: %w", err)
	}
	if event.EventType != apishop.EventTypePremiumUpdated {
		slog.Warn("premium-updated subscriber: unknown event_type, skipping", "event_type", event.EventType)
		return nil
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, event.EventID, event.EventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}
		if err := s.premiumRepo.UpdatePremium(txCtx, event.PlayerID, event.IsPremium, event.PremiumExpiresAt); err != nil {
			return fmt.Errorf("update premium: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("premium-updated subscriber: handler failed",
			"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
		return fmt.Errorf("premium-updated: handler failed: %w", err)
	}
	return nil
}
