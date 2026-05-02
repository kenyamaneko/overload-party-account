package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PremiumUpdatedSubscriber は premium-updated subscription を消費する。
type PremiumUpdatedSubscriber struct {
	stream      port.MessageStream
	premiumRepo port.PlayerPremiumRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewPremiumUpdatedSubscriber は PremiumUpdatedSubscriber を生成する。
func NewPremiumUpdatedSubscriber(
	stream port.MessageStream,
	premiumRepo port.PlayerPremiumRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) *PremiumUpdatedSubscriber {
	return &PremiumUpdatedSubscriber{
		stream:      stream,
		premiumRepo: premiumRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックする。
func (s *PremiumUpdatedSubscriber) Start(ctx context.Context) error {
	slog.Info("premium-updated subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

func (s *PremiumUpdatedSubscriber) processEvent(ctx context.Context, data []byte) error {
	var ev apishop.PremiumUpdatedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("premium-updated subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("premium-updated: bad payload: %w", err)
	}
	if ev.EventType != apishop.EventTypePremiumUpdated {
		slog.Warn("premium-updated subscriber: unknown event_type, acking", "event_type", ev.EventType)
		return nil
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, ev.EventID, ev.EventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}
		if err := s.premiumRepo.UpdatePremium(txCtx, ev.PlayerID, ev.IsPremium, ev.PremiumExpiresAt); err != nil {
			return fmt.Errorf("update premium: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("premium-updated subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("premium-updated: handler failed: %w", err)
	}
	return nil
}
